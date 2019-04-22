package types

import (
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/multiformats/go-multibase"

	"github.com/filecoin-project/go-filecoin/address"
)

func init() {
	cbor.RegisterCborType(PaymentVoucher{})
}

// PaymentVoucher is a voucher for a payment channel that can be transferred off-chain but guarantees a future payment.
type PaymentVoucher struct {
	Channel   ChannelID       `json:"channel"`
	Payer     address.Address `json:"payer"`
	Target    address.Address `json:"target"`
	Amount    AttoFIL         `json:"amount"`
	ValidAt   BlockHeight     `json:"valid_at"`
	Signature Signature       `json:"signature"`
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
