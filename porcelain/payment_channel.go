package porcelain

import (
	"context"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
)

type pclPlumbing interface {
	GetAndMaybeSetDefaultSenderAddress() (address.Address, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// PaymentChannelLs lists payments for a given payer
func PaymentChannelLs(
	ctx context.Context,
	plumbing pclPlumbing,
	fromAddr address.Address,
	payerAddr address.Address,
) (channels map[string]*paymentbroker.PaymentChannel, err error) {
	if fromAddr == (address.Address{}) {
		fromAddr, err = plumbing.GetAndMaybeSetDefaultSenderAddress()
		if err != nil {
			return nil, err
		}
	}

	if payerAddr == (address.Address{}) {
		payerAddr = fromAddr
	}

	values, _, err := plumbing.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"ls",
		payerAddr,
	)
	if err != nil {
		return nil, err
	}

	if err := cbor.DecodeInto(values[0], &channels); err != nil {
		return nil, err
	}

	return channels, nil
}
