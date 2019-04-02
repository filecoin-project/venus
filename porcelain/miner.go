package porcelain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"

	minerActor "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
	w "github.com/filecoin-project/go-filecoin/wallet"
)

// mcAPI is the subset of the plumbing.API that MinerCreate uses.
type mcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error
	MessageSendWithDefaultAddress(ctx context.Context, from, to address.Address, value *types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	WalletDefaultAddress() (address.Address, error)
	WalletGetPubKeyForAddress(addr address.Address) ([]byte, error)
}

// MinerCreate creates a new miner actor for the given account and returns its address.
// It will wait for the the actor to appear on-chain and add set the address to mining.minerAddress in the config.
// TODO: add ability to pass in a KeyInfo to store for signing blocks.
//       See https://github.com/filecoin-project/go-filecoin/issues/1843
func MinerCreate(
	ctx context.Context,
	plumbing mcAPI,
	minerOwnerAddr address.Address,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
	pledge uint64,
	pid peer.ID,
	collateral *types.AttoFIL,
) (_ *address.Address, err error) {
	if minerOwnerAddr == (address.Address{}) {
		minerOwnerAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return nil, err
		}
	}

	ctx = log.Start(ctx, "Node.CreateMiner")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	addr, err := plumbing.ConfigGet("mining.minerAddress")
	if err != nil {
		return nil, err
	}
	if (addr != address.Address{}) {
		return nil, fmt.Errorf("can only have one miner per node")
	}

	pubKey, err := plumbing.WalletGetPubKeyForAddress(minerOwnerAddr)
	if err != nil {
		return nil, err
	}

	smsgCid, err := plumbing.MessageSendWithDefaultAddress(
		ctx,
		minerOwnerAddr,
		address.StorageMarketAddress,
		collateral,
		gasPrice,
		gasLimit,
		"createMiner",
		big.NewInt(int64(pledge)),
		pubKey,
		pid,
	)
	if err != nil {
		return nil, err
	}

	var minerAddr address.Address
	err = plumbing.MessageWait(ctx, smsgCid, func(blk *types.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) (err error) {
		if receipt.ExitCode != uint8(0) {
			return vmErrors.VMExitCodeToError(receipt.ExitCode, storagemarket.Errors)
		}
		minerAddr, err = address.NewFromBytes(receipt.Return[0])
		return err
	})
	if err != nil {
		return nil, err
	}

	if err = plumbing.ConfigSet("mining.minerAddress", minerAddr.String()); err != nil {
		return nil, err
	}

	return &minerAddr, nil
}

// mpcAPI is the subset of the plumbing.API that MinerPreviewCreate uses.
type mpcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	MessagePreview(ctx context.Context, from, to address.Address, method string, params ...interface{}) (types.GasUnits, error)
	NetworkGetPeerID() peer.ID
	WalletDefaultAddress() (address.Address, error)
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
	if fromAddr.Empty() {
		fromAddr, err = plumbing.WalletDefaultAddress()
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
	pubkey := info.PublicKey()

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
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func MinerGetOwnerAddress(ctx context.Context, plumbing mgoaAPI, minerAddr address.Address) (address.Address, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, "getOwner")
	if err != nil {
		return address.Undef, err
	}

	return address.NewFromBytes(res[0])
}

// MinerGetKey queries for the public key of the given miner
func MinerGetKey(ctx context.Context, plumbing mgoaAPI, minerAddr address.Address) ([]byte, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, "getKey")
	if err != nil {
		return []byte{}, err
	}

	return res[0], nil
}

// mgaAPI is the subset of the plumbing.API that MinerGetAsk uses.
type mgaAPI interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// MinerGetAsk queries for an ask of the given miner
func MinerGetAsk(ctx context.Context, plumbing mgaAPI, minerAddr address.Address, askID uint64) (minerActor.Ask, error) {
	ret, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, "getAsk", big.NewInt(int64(askID)))
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
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// MinerGetPeerID queries for the peer id of the given miner
func MinerGetPeerID(ctx context.Context, plumbing mgpidAPI, minerAddr address.Address) (peer.ID, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, "getPeerID")
	if err != nil {
		return "", err
	}

	pid, err := peer.IDFromBytes(res[0])
	if err != nil {
		return peer.ID(""), errors.Wrap(err, "could not decode to peer.ID from message-bytes")
	}
	return pid, nil
}
