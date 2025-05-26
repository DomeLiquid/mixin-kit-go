package kit

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
)

const (
	MixinComputerApiPrefix = "https://computer.mixin.dev"
)

var cli *resty.Client

func init() {
	cli = resty.New().
		SetBaseURL(MixinComputerApiPrefix).
		SetHeader(HeaderContentType, ContentTypeJSON).
		SetTimeout(10 * time.Second)
}

type ComputerClient struct {
	client *resty.Client
}

func NewComputerClient() *ComputerClient {
	return &ComputerClient{
		client: cli,
	}
}

func (c *ComputerClient) SetClient(client *resty.Client) {
	c.client = client
}

type ComputerInfo struct {
	Height   int64   `json:"height"`
	Members  Members `json:"members"`
	Observer string  `json:"observer"`
	Params   Params  `json:"params"`
	Payer    string  `json:"payer"`
	Version  string  `json:"version"`
}

type Members struct {
	AppID     string   `json:"app_id"`
	Members   []string `json:"members"`
	Threshold int64    `json:"threshold"`
}

type Params struct {
	Operation Operation `json:"operation"`
}

type Operation struct {
	Asset string          `json:"asset"`
	Price decimal.Decimal `json:"price"`
}

func (c *ComputerClient) GetComputerInfo(ctx context.Context) (*ComputerInfo, error) {
	var result ComputerInfo

	_, err := cli.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/")
	if err != nil {
		return nil, err
	}

	return &result, nil
}

type ComputerUserInfo struct {
	Id           string `json:"id"`
	MixAddress   string `json:"mix_address"`
	ChainAddress string `json:"chain_address"`
}

func (c *ComputerClient) GetUser(ctx context.Context, addr string) (*ComputerUserInfo, error) {
	var result ComputerUserInfo

	_, err := cli.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/users/" + addr)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

type DepolyedAsset struct {
	AssetID  string `json:"asset_id"`
	Address  string `json:"address"`
	Decimals int64  `json:"decimals"`
	URI      string `json:"uri"`
}

type DepolyedAssets []DepolyedAsset

func (c *ComputerClient) GetDepolyedAssets(ctx context.Context) (DepolyedAssets, error) {
	var result DepolyedAssets

	_, err := cli.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/deployed_assets")
	if err != nil {
		return nil, err
	}

	return result, nil
}

type SystemCallInfo struct {
	Hash         string `json:"hash"`
	ID           string `json:"id"`
	NonceAccount string `json:"nonce_account"`
	Raw          string `json:"raw"`
	Reason       string `json:"reason"` // only for failed
	State        string `json:"state"`  // initial pending done failed
	UserID       string `json:"user_id"`
}

func (c *ComputerClient) SystemCalls(ctx context.Context, id string) (*SystemCallInfo, error) {
	var result SystemCallInfo

	_, err := cli.R().
		SetContext(ctx).
		SetResult(&result).
		Get("/system_calls/" + id)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

type ExternalAsset struct {
	AssetID     string    `json:"asset_id"`
	URI         string    `json:"uri"`
	IconURL     string    `json:"icon_url"`
	CreatedAt   time.Time `json:"created_at"`
	RequestedAt time.Time `json:"requested_at"`
	DeployedAt  time.Time `json:"deployed_at"`
}

func (c *ComputerClient) DeployAsset(ctx context.Context, assets []string) ([]ExternalAsset, error) {
	var result []ExternalAsset

	_, err := cli.R().
		SetContext(ctx).
		SetBody(assets).
		SetResult(&result).
		Post("/deployed_assets")
	if err != nil {
		return nil, err
	}

	return result, nil
}

type NonceAccount struct {
	Mix          string `json:"mix"`
	NonceAddress string `json:"nonce_address"`
	NonceHash    string `json:"nonce_hash"`
}

func (c *ComputerClient) NonceAccounts(ctx context.Context, mix string) (*NonceAccount, error) {
	var result NonceAccount

	_, err := cli.R().
		SetContext(ctx).
		SetBody(map[string]string{"mix": mix}).
		SetResult(&result).
		Post("/nonce_accounts")
	if err != nil {
		return nil, err
	}

	return &result, nil
}

type Fee struct {
	FeedID    string `json:"feed_id"`
	XinAmount string `json:"xin_amount"`
}

func (c *ComputerClient) GetFeeOnXIN(ctx context.Context, sol_amount string) (*Fee, error) {
	var result Fee

	_, err := cli.R().
		SetContext(ctx).
		SetBody(map[string]string{"sol_amount": sol_amount}).
		SetResult(&result).
		Post("/fee")
	if err != nil {
		return nil, err
	}

	return &result, nil
}
