package porcelain

import (
	"context"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
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

type pcvPlumbing interface {
	GetAndMaybeSetDefaultSenderAddress() (address.Address, error)
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
	SignBytes(data []byte, addr address.Address) (types.Signature, error)
}

// PaymentChannelVoucher returns a signed payment channel voucher
func PaymentChannelVoucher(
	ctx context.Context,
	plumbing pcvPlumbing,
	fromAddr address.Address,
	channel *types.ChannelID,
	amount *types.AttoFIL,
	validAt *types.BlockHeight,
) (voucher *paymentbroker.PaymentVoucher, err error) {
	if fromAddr == (address.Address{}) {
		fromAddr, err = plumbing.GetAndMaybeSetDefaultSenderAddress()
		if err != nil {
			return nil, err
		}
	}

	values, _, err := plumbing.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"voucher",
		channel, amount, validAt,
	)
	if err != nil {
		return nil, err
	}

	if err = cbor.DecodeInto(values[0], &voucher); err != nil {
		return nil, err
	}

	sig, err := paymentbroker.SignVoucher(channel, amount, validAt, fromAddr, plumbing)
	if err != nil {
		return nil, err
	}
	voucher.Signature = sig

	return voucher, nil
}
