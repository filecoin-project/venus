package impl

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodePaych struct {
	api          *nodeAPI
	porcelainAPI *porcelain.API
}

func newNodePaych(api *nodeAPI, porcelainAPI *porcelain.API) *nodePaych {
	return &nodePaych{api: api, porcelainAPI: porcelainAPI}
}

func (np *nodePaych) Ls(ctx context.Context, fromAddr, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error) {
	nd := np.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	if payerAddr == (address.Address{}) {
		payerAddr = fromAddr
	}

	values, _, err := np.porcelainAPI.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"ls",
		payerAddr,
	)
	if err != nil {
		return nil, err
	}

	var channels map[string]*paymentbroker.PaymentChannel

	if err := cbor.DecodeInto(values[0], &channels); err != nil {
		return nil, err
	}

	return channels, nil
}

func (np *nodePaych) Voucher(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, amount *types.AttoFIL, validAt *types.BlockHeight) (string, error) {
	nd := np.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return "", err
	}

	values, _, err := np.porcelainAPI.MessageQuery(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"voucher",
		channel, amount, validAt,
	)
	if err != nil {
		return "", err
	}

	var voucher paymentbroker.PaymentVoucher
	if err := cbor.DecodeInto(values[0], &voucher); err != nil {
		return "", err
	}

	sig, err := paymentbroker.SignVoucher(channel, amount, validAt, fromAddr, nd.Wallet)
	if err != nil {
		return "", err
	}
	voucher.Signature = sig

	return voucher.Encode()
}

func (np *nodePaych) Redeem(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, voucherRaw string) (cid.Cid, error) {
	voucher, err := paymentbroker.DecodeVoucher(voucherRaw)
	if err != nil {
		return cid.Undef, err
	}

	return np.porcelainAPI.MessageSendWithDefaultAddress(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		gasPrice,
		gasLimit,
		"redeem",
		voucher.Payer, &voucher.Channel, &voucher.Amount, &voucher.ValidAt, []byte(voucher.Signature),
	)
}

func (np *nodePaych) Close(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, voucherRaw string) (cid.Cid, error) {
	voucher, err := paymentbroker.DecodeVoucher(voucherRaw)
	if err != nil {
		return cid.Undef, err
	}

	return np.porcelainAPI.MessageSendWithDefaultAddress(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		gasPrice,
		gasLimit,
		"close",
		voucher.Payer, &voucher.Channel, &voucher.Amount, &voucher.ValidAt, []byte(voucher.Signature),
	)
}

func (np *nodePaych) Extend(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, channel *types.ChannelID, eol *types.BlockHeight, amount *types.AttoFIL) (cid.Cid, error) {
	return np.porcelainAPI.MessageSendWithDefaultAddress(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		amount,
		gasPrice,
		gasLimit,
		"extend",
		channel, eol,
	)
}
