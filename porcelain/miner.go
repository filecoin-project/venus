package porcelain

import (
	"context"
	"encoding/json"
	"math/big"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"

	minerActor "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
)

// mspPlumbing is the subset of the plumbing.API that MinerSetPrice uses.
type mspPlumbing interface {
	MessageSend(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	ConfigSet(dottedKey string, jsonString string) error
	ConfigGet(dottedPath string) (interface{}, error)
}

// MinerSetPriceResponse collects relevant stats from the set price process
type MinerSetPriceResponse struct {
	Price     *types.AttoFIL
	MinerAddr address.Address
	AddAskCid cid.Cid
	BlockCid  cid.Cid
}

// MinerSetPrice configures the price of storage, then sends an ask advertising that price and waits for it to be mined.
// If minerAddr is empty, the default miner will be used.
// This method is non-transactional in the sense that it will set the price whether or not it creates the ask successfully.
func MinerSetPrice(ctx context.Context, plumbing mspPlumbing, from address.Address, miner address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price *types.AttoFIL, expiry *big.Int) (MinerSetPriceResponse, error) {
	res := MinerSetPriceResponse{
		Price: price,
	}

	// get miner address if not provided
	if miner.Empty() {
		minerValue, err := plumbing.ConfigGet("mining.minerAddress")
		if err != nil {
			return res, errors.Wrap(err, "Could not get miner address in config")
		}
		minerAddr, ok := minerValue.(address.Address)
		if !ok {
			return res, errors.Wrap(err, "Configured miner is not an address")
		}
		miner = minerAddr
	}
	res.MinerAddr = miner

	// set price
	jsonPrice, err := json.Marshal(price)
	if err != nil {
		return res, errors.New("Could not marshal price")
	}
	if err := plumbing.ConfigSet("mining.storagePrice", string(jsonPrice)); err != nil {
		return res, err
	}

	// create ask
	res.AddAskCid, err = plumbing.MessageSend(ctx, from, res.MinerAddr, types.NewZeroAttoFIL(), gasPrice, gasLimit, "addAsk", price, expiry)
	if err != nil {
		return res, errors.Wrap(err, "couldn't send message")
	}

	// wait for ask to be mined
	err = plumbing.MessageWait(ctx, res.AddAskCid, func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) error {
		res.BlockCid = blk.Cid()

		if receipt.ExitCode != uint8(0) {
			return vmErrors.VMExitCodeToError(receipt.ExitCode, minerActor.Errors)
		}
		return nil
	})
	return res, err
}

type mgoaPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func MinerGetOwnerAddress(ctx context.Context, plumbing mgoaPlumbing, minerAddr address.Address) (address.Address, error) {
	res, _, err := plumbing.MessageQuery(ctx, address.Address{}, minerAddr, "getOwner")
	if err != nil {
		return address.Address{}, err
	}

	return address.NewFromBytes(res[0])
}

// MinerGetAsk queries for an ask of the given miner
func MinerGetAsk(ctx context.Context, plumbing mgoaPlumbing, minerAddr address.Address, askID uint64) (minerActor.Ask, error) {
	ret, _, err := plumbing.MessageQuery(ctx, address.Address{}, minerAddr, "getAsk", big.NewInt(int64(askID)))
	if err != nil {
		return minerActor.Ask{}, err
	}

	var ask minerActor.Ask
	if err := cbor.DecodeInto(ret[0], &ask); err != nil {
		return minerActor.Ask{}, err
	}

	return ask, nil
}

// MinerGetPeerID queries for the peer id of the given miner
func MinerGetPeerID(ctx context.Context, plumbing mgoaPlumbing, minerAddr address.Address) (peer.ID, error) {
	res, _, err := plumbing.MessageQuery(ctx, address.Address{}, minerAddr, "getPeerID")
	if err != nil {
		return "", err
	}

	pid, err := peer.IDFromBytes(res[0])
	if err != nil {
		return peer.ID(""), errors.Wrap(err, "could not decode to peer.ID from message-bytes")
	}
	return pid, nil
}
