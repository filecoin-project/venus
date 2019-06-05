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
	// To is the address of the actor to which this predicate is addressed.
	To address.Address `json:"to"`

	// Method is the actor method this predicate will call.
	Method string `json:"method"`

	// Params are the parameters (or a subset of the parameters) used to call the actor method.
	// They must all be individually abi encodable.
	Params []interface{} `json:"params"`
}

// PaymentVoucher is a voucher for a payment channel that can be transferred off-chain but guarantees a future payment.
type PaymentVoucher struct {
	// Channel is the id of this voucher's payment channel.
	Channel ChannelID `json:"channel"`

	// Payer is the address of the account that created the channel.
	Payer address.Address `json:"payer"`

	// Target is the address of the account that will receive funds from the channel.
	Target address.Address `json:"target"`

	// Amount is the FIL this voucher authorizes the target to redeemed from the channel.
	Amount AttoFIL `json:"amount"`

	// ValidAt is the earliest block height at which this voucher is valid.
	ValidAt BlockHeight `json:"valid_at"`

	// Condition defines a optional message that will be called and must return true before this voucher can be redeemed.
	Condition *Predicate `json:"condition"`

	// Signature is the signature of all the data in this voucher.
	Signature Signature `json:"signature"`
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
