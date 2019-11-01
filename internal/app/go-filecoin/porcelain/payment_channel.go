package porcelain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type pclPlumbing interface {
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
	WalletDefaultAddress() (address.Address, error)
}

// PaymentChannelLs lists payments for a given payer
func PaymentChannelLs(
	ctx context.Context,
	plumbing pclPlumbing,
	fromAddr address.Address,
	payerAddr address.Address,
) (channels map[string]*paymentbroker.PaymentChannel, err error) {
	if fromAddr.Empty() {
		fromAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return nil, err
		}
	}

	if payerAddr.Empty() {
		payerAddr = fromAddr
	}

	values, err := plumbing.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		paymentbroker.Ls,
		plumbing.ChainHeadKey(),
		payerAddr,
	)
	if err != nil {
		return nil, err
	}

	if err := encoding.Decode(values[0], &channels); err != nil {
		return nil, err
	}

	return channels, nil
}

type pcvPlumbing interface {
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
	SignBytes(data []byte, addr address.Address) (types.Signature, error)
	WalletDefaultAddress() (address.Address, error)
}

// PaymentChannelVoucher returns a signed payment channel voucher
func PaymentChannelVoucher(
	ctx context.Context,
	plumbing pcvPlumbing,
	fromAddr address.Address,
	channel *types.ChannelID,
	amount types.AttoFIL,
	validAt *types.BlockHeight,
	condition *types.Predicate,
) (voucher *types.PaymentVoucher, err error) {
	if fromAddr.Empty() {
		fromAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return nil, err
		}
	}

	values, err := plumbing.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		paymentbroker.Voucher,
		plumbing.ChainHeadKey(),
		channel, amount, validAt, condition,
	)
	if err != nil {
		return nil, err
	}

	if err = encoding.Decode(values[0], &voucher); err != nil {
		return nil, err
	}

	sig, err := paymentbroker.SignVoucher(channel, amount, validAt, fromAddr, condition, plumbing)
	if err != nil {
		return nil, err
	}
	voucher.Signature = sig

	return voucher, nil
}
