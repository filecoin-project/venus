package impl

import (
	"context"
	"fmt"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmexBtiTTEwwn42Yi6ouKt6VqzpA6wjJgiW1oh9VfaRrup/go-multibase"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type nodePaych struct {
	api *nodeAPI
}

func newNodePaych(api *nodeAPI) *nodePaych {
	return &nodePaych{api: api}
}

func (api *nodePaych) Create(ctx context.Context, fromAddr, target types.Address, eol *types.BlockHeight, amount *types.AttoFIL) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		amount,
		"createChannel",
		target, eol,
	)
}

func (api *nodePaych) Ls(ctx context.Context, fromAddr, payerAddr types.Address) (map[string]*paymentbroker.PaymentChannel, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	if payerAddr == (types.Address{}) {
		payerAddr = fromAddr
	}

	value, err := sendQuery(
		nd,
		fromAddr,
		"ls",
		payerAddr,
	)
	if err != nil {
		return nil, err
	}

	var channels map[string]*paymentbroker.PaymentChannel

	if err := cbor.DecodeInto(value[0], &channels); err != nil {
		return nil, err
	}

	return channels, nil
}

func (api *nodePaych) Voucher(ctx context.Context, fromAddr types.Address, channel *types.ChannelID, amount *types.AttoFIL) (string, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return "", err
	}

	value, err := sendQuery(
		nd,
		fromAddr,
		"voucher",
		channel, amount,
	)
	if err != nil {
		return "", err
	}

	var voucher paymentbroker.PaymentVoucher
	if err := cbor.DecodeInto(value[0], &voucher); err != nil {
		return "", err
	}

	// TODO: really sign this thing
	voucher.Signature = fromAddr.Bytes()

	cborVoucher, err := cbor.DumpObject(voucher)
	if err != nil {
		return "", err
	}

	return multibase.Encode(multibase.Base58BTC, cborVoucher)
}

func (api *nodePaych) Redeem(ctx context.Context, fromAddr types.Address, voucherRaw string) (*cid.Cid, error) {
	voucher, err := decodeVoucher(voucherRaw)
	if err != nil {
		return nil, err
	}

	return api.api.Message().Send(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		"update",
		voucher.Payer, &voucher.Channel, &voucher.Amount, voucher.Signature,
	)
}

func (api *nodePaych) Reclaim(ctx context.Context, fromAddr types.Address, channel *types.ChannelID) (*cid.Cid, error) {
	return api.api.Message().Send(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		"reclaim",
		channel,
	)
}

func (api *nodePaych) Close(ctx context.Context, fromAddr types.Address, voucherRaw string) (*cid.Cid, error) {
	voucher, err := decodeVoucher(voucherRaw)
	if err != nil {
		return nil, err
	}

	return api.api.Message().Send(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		"close",
		voucher.Payer, &voucher.Channel, &voucher.Amount, voucher.Signature,
	)
}

func (api *nodePaych) Extend(ctx context.Context, fromAddr types.Address, channel *types.ChannelID, eol *types.BlockHeight, amount *types.AttoFIL) (*cid.Cid, error) {
	return api.api.Message().Send(
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

func sendQuery(nd *node.Node, fromAddr types.Address, method string, params ...interface{}) ([][]byte, error) {
	args, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, err
	}

	retValue, retCode, err := nd.CallQueryMethod(address.PaymentBrokerAddress, method, args, &fromAddr)
	if err != nil {
		return nil, err
	}

	if retCode != 0 {
		return nil, fmt.Errorf("Non-zero return code executing voucher")
	}

	return retValue, nil
}
