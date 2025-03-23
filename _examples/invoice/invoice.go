package main

import (
	"fmt"

	kit "github.com/DomeLiquid/mixin-kit-go"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
)

func main() {
	iw := kit.NewMixinInvoiceUserId("")
	traceId := mixin.RandomTraceID()
	err := iw.AddEntryIndex(traceId, "64692c23-8971-4cf4-84a7-4dd1271dd887", decimal.NewFromFloat(0.01), "test memo1", nil)
	if err != nil {
		panic(err)
	}

	err = iw.AddEntryIndex(mixin.RandomTraceID(), "c94ac88f-4671-3976-b60a-09064f1811e8", decimal.NewFromFloat(0.01), "test memo2", []uint8{0})
	if err != nil {
		panic(err)
	}

	_ = iw.AddEntryIndex(mixin.RandomTraceID(), "c6d0c728-2624-429b-8e0d-d9d19b6592fa", decimal.NewFromFloat(0.00001), "test memo3", []uint8{1})
	// _ = iw.AddEntryIndex(mixin.RandomTraceID(), "43d61dcd-e413-450d-80b8-101d5e903357", decimal.NewFromFloat(0.01), "test memo4", []uint8{2})
	_ = iw.AddEntryIndex(mixin.RandomTraceID(), "723ef46d-cd07-38af-bc40-988940bbc532", decimal.NewFromFloat(0.01), "test memo4", []uint8{2})

	fmt.Printf("https://mixin.one/pay/%s\n\n", iw.String())
}
