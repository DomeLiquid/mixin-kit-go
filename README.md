# mixin-kit-go

Golang kit for Mixin Network

## Install

```go
go get -u github.com/DomeLiquid/mixin-kit-go
```

## Feature

- A wrapper for the fox-one sdk.
- A toolset to make it easier to interact with Mixin Route


## Transfer CNB

```go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	kit "github.com/DomeLiquid/mixin-kit-go"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
)

const (
	ASSET_CNB = "965e5c6e-434c-3fa9-b780-c50f43cd955c"
)

var (
	config = flag.String("config", "", "keystore file path")
)

/*
!!! Need 1e-8 cnb of your bot to test it
CGO_ENABLED=0 go build -o transfer_cnb && ./transfer_cnb --config ../config_debug.json

Test Tx: https://mixin.space/tx/7b517a40d0ab82524b1fbb87693fdfc3399e28c61dc9222e183d8691d1a00f1d
*/
func main() {
	flag.Parse()

	f, err := os.Open(*config)
	if err != nil {
		log.Panicln(err)
	}

	var config kit.Config
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		log.Panicln(err)
	}

	kitCli, err := kit.NewMixinClientWrapper(&config)
	if err != nil {
		log.Panicf("init client wrapper error: %+v \n", err)
	}
	ctx := context.Background()

	reqId := mixin.RandomTraceID()
	amount := decimal.NewFromFloat(0.00000001)
	var request *mixin.SafeTransactionRequest
	request, err = kitCli.TransferOne(ctx, &kit.TransferOneRequest{
		RequestId: reqId,
		AssetId:   ASSET_CNB,
		Member:    config.AppID,
		Amount:    amount,
		Memo:      "Transferring 1e-8 cnb to myself",
	})
	if err != nil {
		log.Panicf("transfer: %+v \n", err)
	}

	log.Printf("request: %+v \n\n\n", request)

	log.Printf("check Tx: https://mixin.space/tx/%s \n", request.TransactionHash)
}
```