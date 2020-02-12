package porcelain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	minerActor "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// mcAPI is the subset of the plumbing.API that MinerCreate uses.
type mcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
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

	addr, err := plumbing.ConfigGet("mining.minerAddress")
	if err != nil {
		return nil, err
	}
	if (addr != address.Address{}) {
		return nil, fmt.Errorf("can only have one miner per node")
	}

	smsgCid, _, err := plumbing.MessageSend(
		ctx,
		minerOwnerAddr,
		vmaddr.StorageMarketAddress,
		collateral,
		gasPrice,
		gasLimit,
		storagemarket.CreateStorageMiner,
		sectorSize,
		pid,
	)
	if err != nil {
		return nil, err
	}

	var minerAddr address.Address
	err = plumbing.MessageWait(ctx, smsgCid, func(blk *block.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) (err error) {
		if receipt.ExitCode != uint8(0) {
			// Dragons: do we want to have this back?
			return fmt.Errorf("Error executing actor code (exitcode: %d)", receipt.ExitCode)
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
	MessagePreview(ctx context.Context, from, to address.Address, method types.MethodID, params ...interface{}) (types.GasUnits, error)
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

	usedGas, err = plumbing.MessagePreview(
		ctx,
		fromAddr,
		vmaddr.StorageMarketAddress,
		storagemarket.CreateStorageMiner,
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
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
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
	panic("implement me in terms of storage market module")
}

// minerQueryAndDeserialize is the subset of the plumbing.API that provides
// support for sending query messages and getting method signatures.
type minerQueryAndDeserialize interface {
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
	ActorGetStableSignature(ctx context.Context, actorAddr address.Address, method types.MethodID) (vm.ActorMethodSignature, error)
}

// MinerGetOwnerAddress queries for the owner address of the given miner
func MinerGetOwnerAddress(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (address.Address, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, minerActor.GetOwner, plumbing.ChainHeadKey())
	if err != nil {
		return address.Undef, err
	}

	return address.NewFromBytes(res[0])
}

// MinerGetWorkerAddress queries for the worker address of the given miner
func MinerGetWorkerAddress(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, minerActor.GetWorker, baseKey)
	if err != nil {
		return address.Undef, err
	}

	workerAddr, err := address.NewFromBytes(res[0])
	if err != nil {
		return address.Undef, err
	}

	if minerAddr.Protocol() != address.ID {
		return workerAddr, nil
	}

	id, err := address.IDFromAddress(minerAddr)
	if err != nil {
		return address.Undef, err
	}

	res, err = plumbing.MessageQuery(ctx, address.Undef, vmaddr.InitAddress, initactor.GetAddressForActorIDMethodID, baseKey, big.NewInt(int64(id)))
	if err != nil {
		return address.Undef, err
	}

	return address.NewFromBytes(res[0])
}

// queryAndDeserialize is a convenience method. It sends a query message to a
// miner and, based on the method return-type, deserializes to the appropriate
// ABI type.
func queryAndDeserialize(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) (interface{}, error) {
	rets, err := plumbing.MessageQuery(ctx, address.Address{}, minerAddr, method, baseKey, params...)
	if err != nil {
		return nil, errors.Wrapf(err, "'%s' query message failed", method)
	}

	methodSignature, err := plumbing.ActorGetStableSignature(ctx, minerAddr, method)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to acquire '%s' signature", method)
	}

	return methodSignature.ReturnInterface(rets[0])
}

// MinerGetSectorSize queries for the sector size of the given miner.
func MinerGetSectorSize(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (*types.BytesAmount, error) {
	out, err := queryAndDeserialize(ctx, plumbing, minerAddr, minerActor.GetSectorSize, plumbing.ChainHeadKey())
	if err != nil {
		return nil, errors.Wrap(err, "query and deserialize failed")
	}

	sectorSize, ok := out.(*types.BytesAmount)
	if !ok {
		return nil, errors.New("failed to convert returned ABI value")
	}

	return sectorSize, nil
}

// MinerCalculateLateFee calculates the fee due if a miner's PoSt were to be mined at `height`.
func MinerCalculateLateFee(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address, height *types.BlockHeight) (types.AttoFIL, error) {
	out, err := queryAndDeserialize(ctx, plumbing, minerAddr, minerActor.CalculateLateFee, plumbing.ChainHeadKey(), height)
	if err != nil {
		return types.ZeroAttoFIL, errors.Wrap(err, "query and deserialize failed")
	}

	coll, ok := out.(types.AttoFIL)
	if !ok {
		return types.ZeroAttoFIL, errors.New("failed to convert returned ABI value")
	}

	return coll, nil
}

// MinerGetLastCommittedSectorID queries for the id of the last sector committed
// by the given miner.
func MinerGetLastCommittedSectorID(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (uint64, error) {
	out, err := queryAndDeserialize(ctx, plumbing, minerAddr, minerActor.GetLastUsedSectorID, plumbing.ChainHeadKey())
	if err != nil {
		return 0, errors.Wrap(err, "query and deserialize failed")
	}

	lastUsedSectorID, ok := out.(uint64)
	if !ok {
		return 0, errors.New("failed to convert returned ABI value")
	}

	return lastUsedSectorID, nil
}

// mgaAPI is the subset of the plumbing.API that MinerGetAsk uses.
type mgaAPI interface {
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
}

// mgpidAPI is the subset of the plumbing.API that MinerGetPeerID uses.
type mgpidAPI interface {
	ChainHeadKey() block.TipSetKey
	MessageQuery(ctx context.Context, optFrom, to address.Address, method types.MethodID, baseKey block.TipSetKey, params ...interface{}) ([][]byte, error)
}

// MinerGetPeerID queries for the peer id of the given miner
func MinerGetPeerID(ctx context.Context, plumbing mgpidAPI, minerAddr address.Address) (peer.ID, error) {
	res, err := plumbing.MessageQuery(ctx, address.Undef, minerAddr, minerActor.GetPeerID, plumbing.ChainHeadKey())
	if err != nil {
		return "", err
	}

	pid, err := peer.IDFromBytes(res[0])
	if err != nil {
		return peer.ID(""), errors.Wrap(err, "could not decode to peer.ID from message-bytes")
	}
	return pid, nil
}

// MinerProvingWindow contains a miners proving period start and end as well
// as a set of their proving set.
type MinerProvingWindow struct {
	Start      types.BlockHeight
	End        types.BlockHeight
	ProvingSet map[string]types.Commitments
}

// MinerGetProvingWindow gets the proving period and commitments for miner `minerAddr`.
func MinerGetProvingWindow(ctx context.Context, plumbing minerQueryAndDeserialize, minerAddr address.Address) (MinerProvingWindow, error) {
	res, err := plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		minerActor.GetProvingWindow,
		plumbing.ChainHeadKey(),
	)
	if err != nil {
		return MinerProvingWindow{}, errors.Wrap(err, "query ProvingPeriod method failed")
	}

	window, err := abi.Deserialize(res[0], abi.UintArray)
	if err != nil {
		return MinerProvingWindow{}, err
	}
	windowVal := window.Val.([]types.Uint64)

	res, err = plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		minerActor.GetProvingSetCommitments,
		plumbing.ChainHeadKey(),
	)
	if err != nil {
		return MinerProvingWindow{}, errors.Wrap(err, "query SetCommitments method failed")
	}

	sig, err := plumbing.ActorGetStableSignature(ctx, minerAddr, minerActor.GetProvingSetCommitments)
	if err != nil {
		return MinerProvingWindow{}, errors.Wrap(err, "query method failed")
	}
	fmt.Printf("bad bytes: %x\n", res[0])
	commitmentsVal, err := sig.ReturnInterface(res[0])
	if err != nil {
		return MinerProvingWindow{}, errors.Wrap(err, "deserialization failed")
	}
	commitments, ok := commitmentsVal.(map[string]types.Commitments)
	if !ok {
		return MinerProvingWindow{}, errors.New("type assertion failed")
	}

	return MinerProvingWindow{
		Start:      *types.NewBlockHeight(uint64(windowVal[0])),
		End:        *types.NewBlockHeight(uint64(windowVal[1])),
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
		vmaddr.StoragePowerAddress,
		power.GetPowerReport,
		plumbing.ChainHeadKey(),
		minerAddr,
	)
	if err != nil {
		return MinerPower{}, err
	}
	reportValue, err := abi.Deserialize(bytes[0], abi.PowerReport)
	if err != nil {
		return MinerPower{}, err
	}
	powerReport, ok := reportValue.Val.(types.PowerReport)
	if !ok {
		return MinerPower{}, errors.Errorf("invalid report bytes returned from GetPower")
	}
	minerPower := powerReport.ActivePower

	bytes, err = plumbing.MessageQuery(
		ctx,
		address.Undef,
		vmaddr.StoragePowerAddress,
		power.GetTotalPower,
		plumbing.ChainHeadKey(),
	)
	if err != nil {
		return MinerPower{}, err
	}
	total := types.NewBytesAmountFromBytes(bytes[0])

	return MinerPower{
		Power: *minerPower,
		Total: *total,
	}, nil
}

// MinerGetCollateral queries the collateral of a given miner.
func MinerGetCollateral(ctx context.Context, plumbing mgaAPI, minerAddr address.Address) (types.AttoFIL, error) {
	rets, err := plumbing.MessageQuery(
		ctx,
		address.Undef,
		minerAddr,
		minerActor.GetActiveCollateral,
		plumbing.ChainHeadKey(),
	)
	if err != nil {
		return types.AttoFIL{}, err
	}
	return types.NewAttoFILFromBytes(rets[0]), nil
}

// mwapi is the subset of the plumbing.API that MinerSetWorkerAddress use.
type mwapi interface {
	ConfigGet(dottedPath string) (interface{}, error)
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method types.MethodID, params ...interface{}) (cid.Cid, chan error, error)
	MinerGetOwnerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error)
}

// MinerSetWorkerAddress sets the worker address of the miner actor to the provided new address,
// waits for the message to appear on chain and then sets miner.workerAddr config to the new address.
func MinerSetWorkerAddress(
	ctx context.Context,
	plumbing mwapi,
	workerAddr address.Address,
	gasPrice types.AttoFIL,
	gasLimit types.GasUnits,
) (cid.Cid, error) {

	retVal, err := plumbing.ConfigGet("mining.minerAddress")
	if err != nil {
		return cid.Undef, err
	}
	minerAddr, ok := retVal.(address.Address)
	if !ok {
		return cid.Undef, errors.New("problem converting miner address")
	}

	minerOwnerAddr, err := plumbing.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not get miner owner address")
	}

	c, _, err := plumbing.MessageSend(
		ctx,
		minerOwnerAddr,
		minerAddr,
		types.ZeroAttoFIL,
		gasPrice,
		gasLimit,
		minerActor.ChangeWorker,
		workerAddr)
	return c, err
}
