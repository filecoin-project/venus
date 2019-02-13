package porcelain

import (
	"context"
	"encoding/json"
	"fmt"
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
	w "github.com/filecoin-project/go-filecoin/wallet"
)

// mpcAPI is the subset of the plumbing.API that MinerPreviewCreate uses.
type mpcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	GetAndMaybeSetDefaultSenderAddress() (address.Address, error)
	MessagePreview(ctx context.Context, from, to address.Address, method string, params ...interface{}) (types.GasUnits, error)
	NetworkGetPeerID() peer.ID
	WalletFind(address address.Address) (w.Backend, error)
}

// MinerPreviewCreate previews the Gas cost of creating a miner
func MinerPreviewCreate(
	ctx context.Context,
	plumbing mpcAPI,
	fromAddr address.Address,
	pledge uint64,
	pid peer.ID,
	collateral *types.AttoFIL,
) (usedGas types.GasUnits, err error) {
	if fromAddr == (address.Address{}) {
		fromAddr, err = plumbing.GetAndMaybeSetDefaultSenderAddress()
		if err != nil {
			return types.NewGasUnits(0), err
		}
	}

	if pid == "" {
		pid = plumbing.NetworkGetPeerID()
	}

	if _, err := plumbing.ConfigGet("mining.minerAddress"); err != nil {
		return types.NewGasUnits(0), fmt.Errorf("can only have one miner per node")
	}

	ctx = log.Start(ctx, "Node.CreateMiner")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	backend, err := plumbing.WalletFind(fromAddr)
	if err != nil {
		return types.NewGasUnits(0), err
	}
	info, err := backend.GetKeyInfo(fromAddr)
	if err != nil {
		return types.NewGasUnits(0), err
	}
	pubkey, err := info.PublicKey()
	if err != nil {
		return types.NewGasUnits(0), err
	}

	usedGas, err = plumbing.MessagePreview(
		ctx,
		fromAddr,
		address.StorageMarketAddress,
		"createMiner",
		big.NewInt(int64(pledge)),
		pubkey,
		pid,
	)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
	}

	return usedGas, nil
}

// mspAPI is the subset of the plumbing.API that MinerSetPrice uses.
type mspAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedKey string, jsonString string) error
	MessageSendWithDefaultAddress(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

// MinerSetPriceResponse collects relevant stats from the set price process
type MinerSetPriceResponse struct {
	AddAskCid cid.Cid
	BlockCid  cid.Cid
	MinerAddr address.Address
	Price     *types.AttoFIL
}

// MinerSetPrice configures the price of storage, then sends an ask advertising that price and waits for it to be mined.
// If minerAddr is empty, the default miner will be used.
// This method is non-transactional in the sense that it will set the price whether or not it creates the ask successfully.
func MinerSetPrice(ctx context.Context, plumbing mspAPI, from address.Address, miner address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price *types.AttoFIL, expiry *big.Int) (MinerSetPriceResponse, error) {
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
	res.AddAskCid, err = plumbing.MessageSendWithDefaultAddress(ctx, from, res.MinerAddr, types.NewZeroAttoFIL(), gasPrice, gasLimit, "addAsk", price, expiry)
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

// mpspAPI is the subset of the plumbing.API that MinerPreviewSetPrice uses.
type mpspAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedKey string, jsonString string) error
	MessagePreview(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) (types.GasUnits, error)
}

// MinerPreviewSetPrice calculates the amount of Gas needed for a call to MinerSetPrice.
// This method accepts all the same arguments as MinerSetPrice.
func MinerPreviewSetPrice(ctx context.Context, plumbing mpspAPI, from address.Address, miner address.Address, price *types.AttoFIL, expiry *big.Int) (types.GasUnits, error) {
	// get miner address if not provided
	if miner.Empty() {
		minerValue, err := plumbing.ConfigGet("mining.minerAddress")
		if err != nil {
			return types.NewGasUnits(0), errors.Wrap(err, "Could not get miner address in config")
		}
		minerAddr, ok := minerValue.(address.Address)
		if !ok {
			return types.NewGasUnits(0), errors.Wrap(err, "Configured miner is not an address")
		}
		miner = minerAddr
	}

	// set price
	jsonPrice, err := json.Marshal(price)
	if err != nil {
		return types.NewGasUnits(0), errors.New("Could not marshal price")
	}
	if err := plumbing.ConfigSet("mining.storagePrice", string(jsonPrice)); err != nil {
		return types.NewGasUnits(0), err
	}

	// create ask
	usedGas, err := plumbing.MessagePreview(
		ctx,
		from,
		miner,
		"addAsk",
		price,
		expiry,
	)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "couldn't preview message")
	}

	return usedGas, nil
}

// mgoaAPI is the subset of the plumbing.API that MinerGetOwnerAddress uses.
type mgoaAPI interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func MinerGetOwnerAddress(ctx context.Context, plumbing mgoaAPI, minerAddr address.Address) (address.Address, error) {
	res, _, err := plumbing.MessageQuery(ctx, address.Address{}, minerAddr, "getOwner")
	if err != nil {
		return address.Address{}, err
	}

	return address.NewFromBytes(res[0])
}

// mgaAPI is the subset of the plumbing.API that MinerGetAsk uses.
type mgaAPI interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// MinerGetAsk queries for an ask of the given miner
func MinerGetAsk(ctx context.Context, plumbing mgaAPI, minerAddr address.Address, askID uint64) (minerActor.Ask, error) {
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

// mgpidAPI is the subset of the plumbing.API that MinerGetPeerID uses.
type mgpidAPI interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error)
}

// MinerGetPeerID queries for the peer id of the given miner
func MinerGetPeerID(ctx context.Context, plumbing mgpidAPI, minerAddr address.Address) (peer.ID, error) {
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
