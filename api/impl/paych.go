package impl

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	"gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api2"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodePaych struct {
	api  *nodeAPI
	API2 api2.Filecoin
}

func newNodePaych(api *nodeAPI, api2 api2.Filecoin) *nodePaych {
	return &nodePaych{api: api, API2: api2}
}

func (np *nodePaych) Create(ctx context.Context, fromAddr, target address.Address, eol *types.BlockHeight, amount *types.AttoFIL) (cid.Cid, error) {
	return np.API2.MessageSend(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		amount,
		"createChannel",
		target, eol,
	)
}

func (np *nodePaych) Ls(ctx context.Context, fromAddr, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error) {
	nd := np.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	if payerAddr == (address.Address{}) {
		payerAddr = fromAddr
	}

	values, _, err := np.api.Message().Query(
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

func (np *nodePaych) Voucher(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, amount *types.AttoFIL) (string, error) {
	nd := np.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return "", err
	}

	values, _, err := np.api.Message().Query(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"voucher",
		channel, amount,
	)
	if err != nil {
		return "", err
	}

	var voucher paymentbroker.PaymentVoucher
	if err := cbor.DecodeInto(values[0], &voucher); err != nil {
		return "", err
	}

	sig, err := paymentbroker.SignVoucher(channel, amount, fromAddr, nd.Wallet)
	if err != nil {
		return "", err
	}
	voucher.Signature = sig

	cborVoucher, err := cbor.DumpObject(voucher)
	if err != nil {
		return "", err
	}

	return multibase.Encode(multibase.Base58BTC, cborVoucher)
}

func (np *nodePaych) Redeem(ctx context.Context, fromAddr address.Address, voucherRaw string) (cid.Cid, error) {
	voucher, err := decodeVoucher(voucherRaw)
	if err != nil {
		return cid.Undef, err
	}

	return np.API2.MessageSend(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		"redeem",
		voucher.Payer, &voucher.Channel, &voucher.Amount, []byte(voucher.Signature),
	)
}

func (np *nodePaych) Reclaim(ctx context.Context, fromAddr address.Address, channel *types.ChannelID) (cid.Cid, error) {
	return np.API2.MessageSend(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		"reclaim",
		channel,
	)
}

func (np *nodePaych) Close(ctx context.Context, fromAddr address.Address, voucherRaw string) (cid.Cid, error) {
	voucher, err := decodeVoucher(voucherRaw)
	if err != nil {
		return cid.Undef, err
	}

	return np.API2.MessageSend(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		"close",
		voucher.Payer, &voucher.Channel, &voucher.Amount, []byte(voucher.Signature),
	)
}

func (np *nodePaych) Extend(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, eol *types.BlockHeight, amount *types.AttoFIL) (cid.Cid, error) {
	return np.API2.MessageSend(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		amount,
		"extend",
		channel, eol,
	)
}

func decodeVoucher(voucherRaw string) (*paymentbroker.PaymentVoucher, error) {
	_, cborVoucher, err := multibase.Decode(voucherRaw)
	if err != nil {
		return nil, err
	}

	var voucher paymentbroker.PaymentVoucher
	err = cbor.DecodeInto(cborVoucher, &voucher)
	if err != nil {
		return nil, err
	}

	return &voucher, nil
}
