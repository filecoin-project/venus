package api_impl

import (
	"context"
	"fmt"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmexBtiTTEwwn42Yi6ouKt6VqzpA6wjJgiW1oh9VfaRrup/go-multibase"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

type NodePaych struct {
	api *NodeAPI
}

func NewNodePaych(api *NodeAPI) *NodePaych {
	return &NodePaych{api: api}
}

func (api *NodePaych) Create(ctx context.Context, fromAddr, target types.Address, eol *types.BlockHeight, amount *types.AttoFIL) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	params, err := abi.ToEncodedValues(target, eol)
	if err != nil {
		return nil, err
	}

	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, address.PaymentBrokerAddress, amount, "createChannel", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}

func (api *NodePaych) Ls(ctx context.Context, fromAddr, payerAddr types.Address) (map[string]*paymentbroker.PaymentChannel, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	if payerAddr == (types.Address{}) {
		payerAddr = fromAddr
	}

	args, err := abi.ToEncodedValues(payerAddr)
	if err != nil {
		return nil, err
	}

	retValue, retCode, err := nd.CallQueryMethod(address.PaymentBrokerAddress, "ls", args, &fromAddr)
	if err != nil {
		return nil, err
	}

	if retCode != 0 {
		return nil, fmt.Errorf("Non-zero retrurn code executing ls")
	}

	var channels map[string]*paymentbroker.PaymentChannel

	if err := cbor.DecodeInto(retValue[0], &channels); err != nil {
		return nil, err
	}

	return channels, nil
}

func (api *NodePaych) Voucher(ctx context.Context, fromAddr types.Address, channel *types.ChannelID, amount *types.AttoFIL) (string, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return "", err
	}

	args, err := abi.ToEncodedValues(channel, amount)
	if err != nil {
		return "", err
	}

	retValue, retCode, err := nd.CallQueryMethod(address.PaymentBrokerAddress, "voucher", args, &fromAddr)
	if err != nil {
		return "", err
	}

	if retCode != 0 {
		return "", fmt.Errorf("Non-zero return code executing voucher")
	}

	var voucher paymentbroker.PaymentVoucher
	if err := cbor.DecodeInto(retValue[0], &voucher); err != nil {
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

func (api *NodePaych) Redeem(ctx context.Context, fromAddr types.Address, voucherRaw string) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	_, cborVoucher, err := multibase.Decode(voucherRaw)
	if err != nil {
		return nil, err
	}

	var voucher paymentbroker.PaymentVoucher
	err = cbor.DecodeInto(cborVoucher, &voucher)
	if err != nil {
		return nil, err
	}

	params, err := abi.ToEncodedValues(voucher.Payer, &voucher.Channel, &voucher.Amount, voucher.Signature)
	if err != nil {
		return nil, err
	}

	// TODO: Sign this message
	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), "update", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}

func (api *NodePaych) Reclaim(ctx context.Context, fromAddr types.Address, channel *types.ChannelID) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	params, err := abi.ToEncodedValues(channel)
	if err != nil {
		return nil, err
	}

	// TODO: Sign this message
	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), "reclaim", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}

func (api *NodePaych) Close(ctx context.Context, fromAddr types.Address, voucherRaw string) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	_, cborVoucher, err := multibase.Decode(voucherRaw)
	if err != nil {
		return nil, err
	}

	var voucher paymentbroker.PaymentVoucher
	if err := cbor.DecodeInto(cborVoucher, &voucher); err != nil {
		return nil, err
	}

	params, err := abi.ToEncodedValues(voucher.Payer, &voucher.Channel, &voucher.Amount, voucher.Signature)
	if err != nil {
		return nil, err
	}

	// TODO: Sign this message
	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, address.PaymentBrokerAddress, types.NewAttoFILFromFIL(0), "close", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}

func (api *NodePaych) Extend(ctx context.Context, fromAddr types.Address, channel *types.ChannelID, eol *types.BlockHeight, amount *types.AttoFIL) (*cid.Cid, error) {
	nd := api.api.node

	if err := setDefaultFromAddr(&fromAddr, nd); err != nil {
		return nil, err
	}

	params, err := abi.ToEncodedValues(channel, eol)
	if err != nil {
		return nil, err
	}

	// TODO: Sign this message
	msg, err := node.NewMessageWithNextNonce(ctx, nd, fromAddr, address.PaymentBrokerAddress, amount, "extend", params)
	if err != nil {
		return nil, err
	}

	smsg, err := types.NewSignedMessage(*msg, nd.Wallet)
	if err != nil {
		return nil, err
	}

	if err := nd.AddNewMessage(ctx, smsg); err != nil {
		return nil, err
	}

	return smsg.Cid()
}
