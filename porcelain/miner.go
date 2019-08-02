package porcelain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	minerActor "github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/actor/builtin/storagemarket"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	vmErrors "github.com/filecoin-project/go-filecoin/vm/errors"
)

var log = logging.Logger("porcelain")

// mcAPI is the subset of the plumbing.API that MinerCreate uses.
type mcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	WalletDefaultAddress() (address.Address, error)
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
	sectorSize *types.BytesAmount,
	pid peer.ID,
	collateral types.AttoFIL,
) (_ *address.Address, err error) {
	if minerOwnerAddr == (address.Address{}) {
		minerOwnerAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return nil, err
		}
	}

	ctx = log.Start(ctx, "Node.CreateStorageMiner")
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

	smsgCid, err := plumbing.MessageSend(
		ctx,
		minerOwnerAddr,
		address.StorageMarketAddress,
		collateral,
		gasPrice,
		gasLimit,
		"createStorageMiner",
		sectorSize,
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
}

// MinerPreviewCreate previews the Gas cost of creating a miner
func MinerPreviewCreate(
	ctx context.Context,
	plumbing mpcAPI,
	fromAddr address.Address,
	sectorSize *types.BytesAmount,
	pid peer.ID,
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

	ctx = log.Start(ctx, "Node.CreateStorageMiner")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	usedGas, err = plumbing.MessagePreview(
		ctx,
		fromAddr,
		address.StorageMarketAddress,
		"createStorageMiner",
		sectorSize,
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
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method string, params ...interface{}) (cid.Cid, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error
}

// MinerSetPriceResponse collects relevant stats from the set price process
type MinerSetPriceResponse struct {
	AddAskCid cid.Cid
	BlockCid  cid.Cid
	MinerAddr address.Address
	Price     types.AttoFIL
}

// MinerSetPrice configures the price of storage, then sends an ask advertising that price and waits for it to be mined.
// If minerAddr is empty, the default miner will be used.
// This method is non-transactional in the sense that it will set the price whether or not it creates the ask successfully.
func MinerSetPrice(ctx context.Context, plumbing mspAPI, from address.Address, miner address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price types.AttoFIL, expiry *big.Int) (MinerSetPriceResponse, error) {
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
	res.AddAskCid, err = plumbing.MessageSend(ctx, from, res.MinerAddr, types.ZeroAttoFIL, gasPrice, gasLimit, "addAsk", price, expiry)
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
func MinerPreviewSetPrice(ctx context.Context, plumbing mpspAPI, from address.Address, miner address.Address, price types.AttoFIL, expiry *big.Int) (types.GasUnits, error) {
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

// minerQueryAndDeserialize is the subset of the plumbing.API that provides
// support for sending query messages and getting method signatures.
type minerQueryAndDeserialize interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
	ActorGetSignature(ctx context.Context, actorAddr address.Address, method string) (*exec.FunctionSignature, error)
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func MinerGetOwnerAddress(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (address.Address, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, "getOwner")
	if err != nil {
		return address.Undef, err
	}

	return address.NewFromBytes(res[0])
}

// queryAndDeserialize is a convenience method. It sends a query message to a
// miner and, based on the method return-type, deserializes to the appropriate
// ABI type.
func queryAndDeserialize(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address, method string, params ...interface{}) (*abi.Value, error) {
	rets, err := plumbing.MessageQuery(ctx, address.Address{}, minerAddr, method, params...)
	if err != nil {
		return nil, errors.Wrapf(err, "'%s' query message failed", method)
	}

	methodSignature, err := plumbing.ActorGetSignature(ctx, minerAddr, method)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to acquire '%s' signature", method)
	}

	abiValue, err := abi.Deserialize(rets[0], methodSignature.Return[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to deserialize returned value")
	}

	return abiValue, nil
}

// MinerGetSectorSize queries for the sector size of the given miner.
func MinerGetSectorSize(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (*types.BytesAmount, error) {
	abiVal, err := queryAndDeserialize(ctx, plumbing, minerAddr, "getSectorSize")
	if err != nil {
		return nil, errors.Wrap(err, "query and deserialize failed")
	}

	sectorSize, ok := abiVal.Val.(*types.BytesAmount)
	if !ok {
		return nil, errors.New("failed to convert returned ABI value")
	}

	return sectorSize, nil
}

// MinerCalculateLateFee calculates the fee due if a miner's PoSt were to be mined at `height`.
func MinerCalculateLateFee(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address, height *types.BlockHeight) (types.AttoFIL, error) {
	abiVal, err := queryAndDeserialize(ctx, plumbing, minerAddr, "calculateLateFee", height)
	if err != nil {
		return types.ZeroAttoFIL, errors.Wrap(err, "query and deserialize failed")
	}

	coll, ok := abiVal.Val.(types.AttoFIL)
	if !ok {
		return types.ZeroAttoFIL, errors.New("failed to convert returned ABI value")
	}

	return coll, nil
}

// MinerGetLastCommittedSectorID queries for the id of the last sector committed
// by the given miner.
func MinerGetLastCommittedSectorID(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (uint64, error) {
	abiVal, err := queryAndDeserialize(ctx, plumbing, minerAddr, "getLastUsedSectorID")
	if err != nil {
		return 0, errors.Wrap(err, "query and deserialize failed")
	}

	lastUsedSectorID, ok := abiVal.Val.(uint64)
	if !ok {
		return 0, errors.New("failed to convert returned ABI value")
	}

	return lastUsedSectorID, nil
}

// MinerGetWorker queries for the public key of the given miner
func MinerGetWorker(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (address.Address, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, "getWorker")
	if err != nil {
		return address.Undef, err
	}

	return address.NewFromBytes(res[0])
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

// MinerProvingPeriod contains a miners proving period start and end as well
// as a set of their proving set.
type MinerProvingPeriod struct {
	Start      types.BlockHeight
	End        types.BlockHeight
	ProvingSet map[string]types.Commitments
}

// MinerGetProvingPeriod gets the proving period and commitments for miner `minerAddr`.
func MinerGetProvingPeriod(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (MinerProvingPeriod, error) {
	res, err := plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		"getProvingPeriod",
	)
	if err != nil {
		return MinerProvingPeriod{}, errors.Wrap(err, "query ProvingPeriod method failed")
	}
	start, end := types.NewBlockHeightFromBytes(res[0]), types.NewBlockHeightFromBytes(res[1])

	res, err = plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		"getProvingSetCommitments",
	)
	if err != nil {
		return MinerProvingPeriod{}, errors.Wrap(err, "query SetCommitments method failed")
	}

	sig, err := plumbing.ActorGetSignature(ctx, minerAddr, "getProvingSetCommitments")
	if err != nil {
		return MinerProvingPeriod{}, errors.Wrap(err, "query method failed")
	}

	commitmentsVal, err := abi.Deserialize(res[0], sig.Return[0])
	if err != nil {
		return MinerProvingPeriod{}, errors.Wrap(err, "deserialization failed")
	}
	commitments, ok := commitmentsVal.Val.(map[string]types.Commitments)
	if !ok {
		return MinerProvingPeriod{}, errors.New("type assertion failed")
	}

	return MinerProvingPeriod{
		Start:      *start,
		End:        *end,
		ProvingSet: commitments,
	}, nil
}

// MinerPower contains a miners power and the total power of the network
type MinerPower struct {
	Power types.BytesAmount
	Total types.BytesAmount
}

// MinerGetPower queries the power of a given miner.
func MinerGetPower(ctx context.Context, plumbing mgaAPI, minerAddr address.Address) (MinerPower, error) {
	bytes, err := plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		"getPower",
	)
	if err != nil {
		return MinerPower{}, err
	}
	power := types.NewBytesAmountFromBytes(bytes[0])

	bytes, err = plumbing.MessageQuery(
		ctx,
		address.Undef,
		address.StorageMarketAddress,
		"getTotalStorage",
	)
	if err != nil {
		return MinerPower{}, err
	}
	total := types.NewBytesAmountFromBytes(bytes[0])

	return MinerPower{
		Power: *power,
		Total: *total,
	}, nil
}

// MinerGetCollateral queries the collateral of a given miner.
func MinerGetCollateral(ctx context.Context, plumbing mgaAPI, minerAddr address.Address) (types.AttoFIL, error) {
	rets, err := plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		"getActiveCollateral",
	)
	if err != nil {
		return types.AttoFIL{}, err
	}
	return types.NewAttoFILFromBytes(rets[0]), nil
}
