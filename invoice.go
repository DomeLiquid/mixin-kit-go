package kit

import (
	"fmt"

	"github.com/MixinNetwork/bot-api-go-client/v3"
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/shopspring/decimal"
)

type MixinInvoiceWrapper struct {
	Invoice *bot.MixinInvoice
}

func NewMixinInvoiceUserId(uid string) *MixinInvoiceWrapper {
	mi := bot.NewMixinInvoice(bot.NewUUIDMixAddress([]string{uid}, 1).String())
	return &MixinInvoiceWrapper{mi}
}

func (m *MixinInvoiceWrapper) AddEntryHash(traceId, assetId string, amount decimal.Decimal, memo string, hashReferences []crypto.Hash) error {
	if len(memo) > common.ExtraSizeGeneralLimit {
		return fmt.Errorf("memo too long")
	}

	if len(hashReferences) > common.ReferencesCountLimit {
		return fmt.Errorf("indexReferences too long")
	}

	m.Invoice.AddEntry(traceId, assetId, common.NewIntegerFromString(amount.String()), []byte(memo), nil, hashReferences)

	return nil
}

func (m *MixinInvoiceWrapper) AddEntryIndex(traceId, assetId string, amount decimal.Decimal, memo string, indexReferences []uint8) error {
	if len(memo) > common.ExtraSizeGeneralLimit {
		return fmt.Errorf("memo too long")
	}

	if len(indexReferences) > common.ReferencesCountLimit {
		return fmt.Errorf("indexReferences too long")
	}

	for _, ir := range indexReferences {
		if int(ir) >= len(m.Invoice.Entries) {
			return fmt.Errorf("indexReferences too long")
		}
	}
	m.Invoice.AddEntry(traceId, assetId, common.NewIntegerFromString(amount.String()), []byte(memo), indexReferences, nil)
	return nil
}

func (m *MixinInvoiceWrapper) AddStorageEntry(traceId string, extra []byte) {
	m.Invoice.AddStorageEntry(traceId, extra)
}

func (m *MixinInvoiceWrapper) String() string {
	return m.Invoice.String()
}

func NewMixinInvoiceWrapperFromString(s string) (*MixinInvoiceWrapper, error) {
	mi, err := bot.NewMixinInvoiceFromString(s)
	if err != nil {
		return nil, err
	}
	return &MixinInvoiceWrapper{mi}, nil
}
