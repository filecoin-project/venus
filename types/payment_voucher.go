package types

import (
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/multiformats/go-multibase"

	"github.com/filecoin-project/go-filecoin/address"
)

func init() {
	cbor.RegisterCborType(Predicate{})
	cbor.RegisterCborType(PaymentVoucher{})
}

// Predicate is an optional message that is sent to another actor and must return true for the voucher to be valid.
type Predicate struct {
	To     address.Address `json:"to"`
	Method string          `json:"method"`
	Params []byte          `json:"params"`
}

// PaymentVoucher is a voucher for a payment channel that can be transferred off-chain but guarantees a future payment.
type PaymentVoucher struct {
	Channel   ChannelID       `json:"channel"`
	Payer     address.Address `json:"payer"`
	Target    address.Address `json:"target"`
	Amount    AttoFIL         `json:"amount"`
	ValidAt   BlockHeight     `json:"valid_at"`
	Signature Signature       `json:"signature"`
	Condition *Predicate      `json:"condition"`
}

// DecodeVoucher creates a *PaymentVoucher from a base58, Cbor-encoded one
func DecodeVoucher(voucherRaw string) (*PaymentVoucher, error) {
	_, cborVoucher, err := multibase.Decode(voucherRaw)
	if err != nil {
		return nil, err
	}

	var voucher PaymentVoucher
	err = cbor.DecodeInto(cborVoucher, &voucher)
	if err != nil {
		return nil, err
	}

	return &voucher, nil
}

// Encode creates a base58, Cbor-encoded string representation
func (voucher *PaymentVoucher) Encode() (string, error) {
	cborVoucher, err := cbor.DumpObject(voucher)
	if err != nil {
		return "", err
	}

	return multibase.Encode(multibase.Base58BTC, cborVoucher)
}
