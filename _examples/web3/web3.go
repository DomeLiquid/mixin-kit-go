package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/DomeLiquid/mixin-kit-go/_examples"
	"github.com/shopspring/decimal"

	kit "github.com/DomeLiquid/mixin-kit-go"
	"github.com/fox-one/mixin-sdk-go/v2"
)

var (
	config = flag.String("config", "", "keystore file path")
)

/*
CGO_ENABLED=0  go build -o web3 && ./web3 --config ../config_debug.json
*/
func main() {
	flag.Parse()

	f, err := os.Open(*config)
	if err != nil {
		log.Panicln(err)
	}

	var config _examples.Config
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		log.Panicln(err)
	}

	kitCli, err := kit.NewMixinClientWrapper(&mixin.Keystore{
		AppID:             config.AppID,
		SessionID:         config.SessionID,
		ServerPublicKey:   config.ServerPublicKey,
		SessionPrivateKey: config.SessionPrivateKey,
	}, config.SpendKey)
	if err != nil {
		log.Panicf("init client wrapper error: %+v \n", err)
	}
	ctx := context.Background()

	// 1, 查询支持的 tokens
	tokens, err := kitCli.Web3Tokens(ctx)
	if err != nil {
		log.Fatal("[GetWeb3Tokens] error: %+v\n", err)
	}

	for _, token := range tokens {
		log.Printf("token: %+v\n", token)
	}

	// 2. 问价 0.1BTC -> ? SOL
	quoteView, err := kitCli.Web3Quote(ctx, kit.QuoteRequest{
		InputMint:  "c6d0c728-2624-429b-8e0d-d9d19b6592fa", // BTC
		OutputMint: "64692c23-8971-4cf4-84a7-4dd1271dd887", // SOL
		Amount:     decimal.NewFromFloat(0.1),
	})
	if err != nil {
		log.Fatal("web3 quote error: %+v\n", err)
	}
	log.Printf("quote view: %+v \n\n", quoteView)

	// 3. 创建订单 0.01 BTC -> ? SOL
	swapView, err := kitCli.Web3Swap(ctx, kit.SwapRequest{
		Payer:       kitCli.ClientID,
		InputMint:   "c6d0c728-2624-429b-8e0d-d9d19b6592fa", // BTC
		OutputMint:  "64692c23-8971-4cf4-84a7-4dd1271dd887", // SOL
		InputAmount: decimal.NewFromFloat(0.01),
		Payload:     quoteView.Payload,
	})
	if err != nil {
		log.Fatal("web3 quote error: %+v\n", err)
	}
	log.Printf("swap view: %+v \n\n", swapView)

	// 3.1 解析订单信息
	swapTx, err := swapView.DecodeTx()
	if err != nil {
		log.Fatal("web3 decode tx error: %+v\n", err)
	}

	// wait... then query order status
	// 3.2 查询订单状态
	swapOrder, err := kitCli.GetWeb3SwapOrder(ctx, swapTx.OrderId)
	if err != nil {
		log.Fatal("web3 swap order error: %+v\n", err)
	}
	log.Printf("swap order: %+v \n\n", swapOrder)

	//  4. 向 Mixin Route 支付订单
	/*
		safeTransactionRequest, err := kitCli.TransferOne(ctx, &kit.TransferOneRequest{
			RequestId: swapTx.Trace,
			AssetId:   swapTx.Asset,
			Member:    swapTx.Payee,
			Amount:    swapTx.Amount,
			Memo:      swapTx.Memo,
		})
		if err != nil {
			log.Fatal("[TransferOne] error : %+v\n", err)
		}

		log.Printf("safeTransactionRequest: %+v \n\n", safeTransactionRequest)
	*/
	log.Printf("Waiting to check orderId: [%s] status...", swapTx.OrderId)

	time.Sleep(10 * time.Second)
	// 5. 等待 10s 再次查订单状态
	// wait... then query order status
	swapOrder, err = kitCli.GetWeb3SwapOrder(ctx, swapTx.OrderId)
	if err != nil {
		log.Fatal("web3 swap order error: %+v\n", err)
	}
	log.Printf("swap order: %+v \n\n", swapOrder)
}
