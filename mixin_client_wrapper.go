package kit

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"

	bot "github.com/MixinNetwork/bot-api-go-client/v3"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/go-resty/resty/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/shopspring/decimal"
)

var (
	ErrConfigNil             = errors.New("config is nil")
	ErrNotEnoughUtxos        = errors.New("not enough utxos")
	ErrInscriptionNotFound   = errors.New("inscription not found")
	ErrMultInscriptionsFound = errors.New("multiple inscriptions found")
	ErrMaxUtxoExceeded       = errors.New("maximum utxo exceeded")
)

const (
	AGGREGRATE_UTXO_MEMO = "arrgegate utxos"
	MAX_UTXO_NUM         = 255
)

type ClientWrapper struct {
	*mixin.Client
	Web3Client

	user     *mixin.User
	SpendKey mixinnet.Key
	client   *resty.Client

	transferMutex sync.Mutex
}

// GenUuidFromStrings
func GenUuidFromStrings(strs ...string) string {
	var str string
	for _, s := range strs {
		str += s
	}
	return uuid.NewV5(uuid.NamespaceOID, str).String()
}

func NewMixinClientWrapper(keystore *mixin.Keystore, spendKeyStr string) (*ClientWrapper, error) {
	client, err := mixin.NewFromKeystore(keystore)
	if err != nil {
		return nil, err
	}
	user, err := client.UserMe(context.Background())
	if err != nil {
		return nil, err
	}

	spendKey, err := mixinnet.ParseKeyWithPub(spendKeyStr, user.SpendPublicKey)
	if err != nil {
		return nil, err
	}
	su := &bot.SafeUser{
		UserId:            keystore.ClientID,
		SessionId:         keystore.SessionID,
		SessionPrivateKey: keystore.SessionPrivateKey,
		ServerPublicKey:   keystore.ServerPublicKey,
		SpendPrivateKey:   spendKey.String(),
	}

	logger := slog.Default()
	botCli := bot.NewDefaultClient(su, logger)

	clientWrapper := &ClientWrapper{
		Web3Client: NewWeb3Client(botCli),
		client: resty.New().
			SetHeader("Content-Type", "application/json").
			SetBaseURL(MixinRouteApiPrefix).
			SetTimeout(10 * time.Second),
		Client:        client,
		SpendKey:      spendKey,
		user:          user,
		transferMutex: sync.Mutex{},
	}

	return clientWrapper, nil
}

type Web3Response[T any] struct {
	Data T `json:"data"`
}

/*
GET /markets/:coin_idcoin_id: string, coin_id from GET /markets. OR mixin asset idresponse:
*/
func (m *ClientWrapper) GetAssetInfo(ctx context.Context, assetId string) (*MarketAssetInfo, error) {
	var response Web3Response[MarketAssetInfo]
	_, err := m.client.R().SetContext(ctx).SetPathParams(map[string]string{
		"coin_id": assetId,
	}).SetResult(&response).Get("/markets/{coin_id}")
	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

/*
GET /markets/:coin_id/price-history?type=${type}paramdescriptioncoin_idcoin_id from GET /markets, or mixin asset idtype1D, 1W, 1M, YTD, ALLresponse:
*/
func (m *ClientWrapper) GetPriceHistory(ctx context.Context, assetId string, t HistoryPriceType) (*HistoricalPrice, error) {
	var response Web3Response[HistoricalPrice]

	_, err := m.client.R().
		SetContext(ctx).
		SetPathParams(map[string]string{
			"coin_id": assetId,
		}).
		SetQueryParam("type", t.String()).
		SetResult(&response).
		Get("/markets/{coin_id}/price-history")

	if err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (m *ClientWrapper) Web3Tokens(ctx context.Context) (tokens []TokenView, err error) {
	var response Web3Response[[]TokenView]

	err = m.Web3Client.Get(
		ctx,
		"/web3/tokens",
		"source=mixin",
		&response,
	)

	return response.Data, err
}

func (m *ClientWrapper) Web3Quote(ctx context.Context, req QuoteRequest) (resp QuoteResponseView, err error) {
	var response Web3Response[QuoteResponseView]

	err = m.Web3Client.DoRequest(
		ctx,
		"GET",
		"/web3/quote",
		req.ToQuery(),
		nil,
		&response,
	)

	return response.Data, err
}

func (m *ClientWrapper) Web3Swap(ctx context.Context, req SwapRequest) (resp SwapResponseView, err error) {
	var response Web3Response[SwapResponseView]
	err = m.Web3Client.Post(
		ctx,
		"/web3/swap",
		req,
		&response,
	)

	return response.Data, err
}

func (m *ClientWrapper) GetWeb3SwapOrder(ctx context.Context, orderId string) (order SwapOrder, err error) {
	var result struct {
		Data SwapOrder `json:"data"`
	}

	err = m.Web3Client.Get(
		ctx,
		"/web3/swap/orders/"+orderId,
		"",
		&result,
	)

	return result.Data, err
}

type TransferOneRequest struct {
	RequestId string
	AssetId   string

	Member string
	Amount decimal.Decimal
	Memo   string
}

type MemberAmount struct {
	Member []string
	Amount decimal.Decimal
}

type TransferManyRequest struct {
	RequestId string
	AssetId   string

	MemberAmount []MemberAmount
	Memo         string
}

type InscriptionTransferRequest struct {
	RequestId   string
	AssetId     string
	Inscription string

	Memo   string
	Member string
}

// 主动聚合utxos 至 utxo 数量不超过 255 个
func (c *ClientWrapper) SyncArrgegateUtxos(ctx context.Context, assetId string) (utxos []*mixin.SafeUtxo, err error) {
	c.transferMutex.Lock()
	defer c.transferMutex.Unlock()

	utxos = make([]*mixin.SafeUtxo, 0)

	for {
		requestId := mixin.RandomTraceID()
		utxos, err = c.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Asset:     assetId,
			State:     mixin.SafeUtxoStateUnspent,
			Threshold: 1,
		})

		if err != nil {
			return
		}

		if len(utxos) <= MAX_UTXO_NUM {
			// 主动聚合完成
			break
		}

		// 将utxos分割成 255 个大小的数组
		utxoSlice := make([]*mixin.SafeUtxo, 0, MAX_UTXO_NUM)
		utxoSliceAmount := decimal.Zero
		for i := 0; i < len(utxos); i++ {
			if utxos[i].InscriptionHash.HasValue() { // skip inscription
				continue
			}
			utxoSlice = append(utxoSlice, utxos[i])
			utxoSliceAmount = utxoSliceAmount.Add(utxos[i].Amount)
		}

		// 2: build transaction
		b := mixin.NewSafeTransactionBuilder(utxoSlice)
		b.Memo = AGGREGRATE_UTXO_MEMO
		var tx *mixinnet.Transaction
		tx, err = c.MakeTransaction(ctx, b, []*mixin.TransactionOutput{
			{
				Address: mixin.RequireNewMixAddress([]string{c.ClientID}, 1),
				Amount:  utxoSliceAmount,
			},
		})
		if err != nil {
			return nil, err
		}
		var raw string
		raw, err = tx.Dump()
		if err != nil {
			return
		}

		// 3. create transaction
		var request *mixin.SafeTransactionRequest
		request, err = c.SafeCreateTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
			RequestID:      requestId,
			RawTransaction: raw,
		})
		if err != nil {
			return
		}

		// 4. sign transaction
		err = mixin.SafeSignTransaction(
			tx,
			c.SpendKey,
			request.Views,
			0,
		)
		if err != nil {
			return
		}

		var signedRaw string
		signedRaw, err = tx.Dump()
		if err != nil {
			return
		}

		// 5. submit transaction
		_, err = c.SafeSubmitTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
			RequestID:      requestId,
			RawTransaction: signedRaw,
		})
		if err != nil {
			return
		}

		// 重试读取交易状态
		const defaultMaxRetryTimes = 3
		retryTimes := 0
		for {
			if retryTimes >= defaultMaxRetryTimes {
				break
			}

			retryTimes++
			time.Sleep(time.Second * time.Duration(retryTimes))
			_, err = c.SafeReadTransactionRequest(ctx, requestId)
			if err != nil {
				return
			} else {
				break
			}
		}

		// wait 250ms ...
		time.Sleep(time.Second >> 2)
	}
	return
}

func (c *ClientWrapper) TransferOne(ctx context.Context, req *TransferOneRequest) (*mixin.SafeTransactionRequest, error) {
	var err error
	var utxos []*mixin.SafeUtxo

	utxos, err = c.SyncArrgegateUtxos(ctx, req.AssetId)
	if err != nil {
		return nil, err
	}

	c.transferMutex.Lock()
	defer c.transferMutex.Unlock()

	for i := 0; i < 3 && len(utxos) == 0; i++ {
		utxos, _ = c.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Asset:     req.AssetId,
			State:     mixin.SafeUtxoStateUnspent,
			Threshold: 1,
			Limit:     500,
		})
		if len(utxos) > 0 {
			break
		}
		time.Sleep(time.Second << 1)
	}

	if len(utxos) == 0 {
		return nil, ErrNotEnoughUtxos
	}

	for i := 0; i < len(utxos); i++ {
		if utxos[i].InscriptionHash.HasValue() {
			utxos = append(utxos[:i], utxos[i+1:]...)
			i--
		}
	}

	// 1: select utxos
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Amount.LessThanOrEqual(utxos[j].Amount)
	})
	var useAmount decimal.Decimal
	var useUtxos []*mixin.SafeUtxo

	for _, utxo := range utxos {
		useAmount = useAmount.Add(utxo.Amount)
		useUtxos = append(useUtxos, utxo)

		if len(useUtxos) > MAX_UTXO_NUM {
			useUtxos = useUtxos[1:]
			useAmount = useAmount.Sub(utxos[0].Amount)
		}

		if useAmount.GreaterThanOrEqual(req.Amount) {
			break
		}
	}

	if useAmount.LessThan(req.Amount) {
		return nil, ErrNotEnoughUtxos
	}

	// 2: build transaction
	b := mixin.NewSafeTransactionBuilder(useUtxos)
	b.Memo = req.Memo

	txOutout := &mixin.TransactionOutput{
		Address: mixin.RequireNewMixAddress([]string{req.Member}, 1),
		Amount:  req.Amount,
	}

	tx, err := c.MakeTransaction(ctx, b, []*mixin.TransactionOutput{txOutout})
	if err != nil {
		return nil, err
	}

	raw, err := tx.Dump()
	if err != nil {
		return nil, err
	}

	// 3. create transaction
	request, err := c.SafeCreateTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      req.RequestId,
		RawTransaction: raw,
	})
	if err != nil {
		return nil, err
	}
	// 4. sign transaction
	err = mixin.SafeSignTransaction(
		tx,
		c.SpendKey,
		request.Views,
		0,
	)
	if err != nil {
		return nil, err
	}
	signedRaw, err := tx.Dump()
	if err != nil {
		return nil, err
	}

	// 5. submit transaction
	_, err = c.SafeSubmitTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      req.RequestId,
		RawTransaction: signedRaw,
	})
	if err != nil {
		return nil, err
	}

	// 6. read transaction
	req1, err := c.SafeReadTransactionRequest(ctx, req.RequestId)
	if err != nil {
		return nil, err
	}
	return req1, nil
}

/* req.MemberAmount length No limit */
func (m *ClientWrapper) TransferManyN(ctx context.Context, req *TransferManyRequest) error {
	if len(req.MemberAmount) < MAX_UTXO_NUM {
		_, err := m.TransferMany(ctx, req)
		if err != nil {
			return err
		}
	} else {
		memberAmountArray := buildTransferMany(req.MemberAmount)
		for i, memberAmount := range memberAmountArray {
			req := &TransferManyRequest{
				RequestId:    GenUuidFromStrings(req.RequestId, strconv.Itoa(i)),
				AssetId:      req.AssetId,
				MemberAmount: memberAmount,
				Memo:         req.Memo,
			}

			_, err := m.TransferMany(ctx, req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// req.MemberAmount max 255
func (m *ClientWrapper) TransferMany(ctx context.Context, req *TransferManyRequest) (*mixin.SafeTransactionRequest, error) {
	if len(req.MemberAmount) > MAX_UTXO_NUM {
		return nil, ErrMaxUtxoExceeded
	}

	var utxos []*mixin.SafeUtxo
	var err error
	utxos, err = m.SyncArrgegateUtxos(ctx, req.AssetId)
	if err != nil {
		return nil, err
	}

	m.transferMutex.Lock()
	defer m.transferMutex.Unlock()

	totalAmount := decimal.Zero
	for _, item := range req.MemberAmount {
		totalAmount = totalAmount.Add(item.Amount)
	}

	retryCount := 0
	for len(utxos) == 0 && retryCount < 3 {
		// 1. 将utxos聚合
		utxos, _ = m.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Asset:     req.AssetId,
			State:     mixin.SafeUtxoStateUnspent,
			Threshold: 1,
			Limit:     500,
		})
		time.Sleep(time.Second << retryCount)
		retryCount++
	}
	if len(utxos) == 0 {
		return nil, ErrNotEnoughUtxos
	}

	// 1: select utxos
	sort.Slice(utxos, func(i, j int) bool {
		return utxos[i].Amount.LessThanOrEqual(utxos[j].Amount)
	})

	var useAmount decimal.Decimal
	var useUtxos []*mixin.SafeUtxo
	for _, utxo := range utxos {
		useAmount = useAmount.Add(utxo.Amount)
		useUtxos = append(useUtxos, utxo)

		if len(useUtxos) > MAX_UTXO_NUM {
			useUtxos = useUtxos[1:]
			useAmount = useAmount.Sub(utxos[0].Amount)
		}

		if useAmount.GreaterThanOrEqual(totalAmount) {
			break
		}
	}

	if useAmount.LessThan(totalAmount) {
		return nil, ErrNotEnoughUtxos
	}

	// 2: build transaction
	b := mixin.NewSafeTransactionBuilder(useUtxos)
	b.Memo = req.Memo

	txOutout := make([]*mixin.TransactionOutput, len(req.MemberAmount))
	for i := 0; i < len(req.MemberAmount); i++ {
		txOutout[i] = &mixin.TransactionOutput{
			Address: mixin.RequireNewMixAddress(req.MemberAmount[i].Member, byte(len(req.MemberAmount[i].Member))),
			Amount:  req.MemberAmount[i].Amount,
		}
	}

	tx, err := m.MakeTransaction(ctx, b, txOutout)
	if err != nil {
		return nil, err
	}

	raw, err := tx.Dump()
	if err != nil {
		return nil, err
	}

	// 3. create transaction
	request, err := m.SafeCreateTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      req.RequestId,
		RawTransaction: raw,
	})
	if err != nil {
		return nil, err
	}
	// 4. sign transaction
	err = mixin.SafeSignTransaction(
		tx,
		m.SpendKey,
		request.Views,
		0,
	)
	if err != nil {
		return nil, err
	}
	signedRaw, err := tx.Dump()
	if err != nil {
		return nil, err
	}

	// 5. submit transaction
	_, err = m.SafeSubmitTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      req.RequestId,
		RawTransaction: signedRaw,
	})
	if err != nil {
		return nil, err
	}

	// 6. read transaction
	req1, err := m.SafeReadTransactionRequest(ctx, req.RequestId)
	if err != nil {
		return nil, err
	}
	return req1, nil
}

// 一个功能函数，将一个数组中的多个元素切分成 n个数组，每个数组长度最多不超过255个
func buildTransferMany(memberAmounts []MemberAmount) [][]MemberAmount {
	result := make([][]MemberAmount, (len(memberAmounts)+MAX_UTXO_NUM-1)/MAX_UTXO_NUM)
	for i := 0; i < len(result); i++ {
		start := i * MAX_UTXO_NUM
		end := (i + 1) * MAX_UTXO_NUM
		if end > len(memberAmounts) {
			end = len(memberAmounts)
		}
		result[i] = memberAmounts[start:end]
	}
	return result
}

func (m *ClientWrapper) InscriptionTransfer(ctx context.Context, req *InscriptionTransferRequest) (req1 *mixin.SafeTransactionRequest, err error) {
	var utxos []*mixin.SafeUtxo
	utxos, err = m.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
		Asset:     req.AssetId,
		State:     mixin.SafeUtxoStateUnspent,
		Threshold: 1,
		Limit:     500,
	})
	if err != nil {
		return
	}

	for i := range utxos {
		if utxos[i].InscriptionHash.String() == req.Inscription {
			utxos = []*mixin.SafeUtxo{utxos[i]}
		}
	}

	if len(utxos) == 0 {
		err = ErrInscriptionNotFound
		return
	}

	if len(utxos) > 1 {
		err = ErrMultInscriptionsFound
		return
	}

	b := mixin.NewSafeTransactionBuilder(utxos)
	b.Memo = req.Memo

	txOutout := &mixin.TransactionOutput{
		Address: mixin.RequireNewMixAddress([]string{req.Member}, 1),
		Amount:  utxos[0].Amount,
	}

	tx, err := m.MakeTransaction(ctx, b, []*mixin.TransactionOutput{txOutout})
	if err != nil {
		return
	}

	raw, err := tx.Dump()
	if err != nil {
		return
	}

	// 3. create transaction
	request, err := m.SafeCreateTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      req.RequestId,
		RawTransaction: raw,
	})
	if err != nil {
		return
	}
	// 4. sign transaction
	err = mixin.SafeSignTransaction(
		tx,
		m.SpendKey,
		request.Views,
		0,
	)
	if err != nil {
		return
	}
	signedRaw, err := tx.Dump()
	if err != nil {
		return
	}

	// 5. submit transaction
	_, err = m.SafeSubmitTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      req.RequestId,
		RawTransaction: signedRaw,
	})
	if err != nil {
		return
	}

	// 6. read transaction
	req1, err = m.SafeReadTransactionRequest(ctx, req.RequestId)
	return
}
