package porcelain

import (
	"context"
	"fmt"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	cid "github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

// mcAPI is the subset of the plumbing.API that MinerCreate uses.
type mcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error
	WalletDefaultAddress() (address.Address, error)
}

type MinerStateView interface {
	MinerControlAddresses(ctx context.Context, maddr address.Address) (owner, worker address.Address, err error)
	MinerPeerID(ctx context.Context, maddr address.Address) (peer.ID, error)
	MinerSectorSize(ctx context.Context, maddr address.Address) (abi.SectorSize, error)
	MinerProvingPeriod(ctx context.Context, maddr address.Address) (start abi.ChainEpoch, end abi.ChainEpoch, failureCount int, err error)
	NetworkTotalPower(ctx context.Context) (abi.StoragePower, error)
	MinerClaimedPower(ctx context.Context, miner address.Address) (abi.StoragePower, error)
	MinerPledgeCollateral(ctx context.Context, miner address.Address) (locked abi.TokenAmount, total abi.TokenAmount, err error)
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
	sectorSize abi.SectorSize,
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
	if addr != address.Undef {
		return nil, fmt.Errorf("can only have one miner per node")
	}

	params := power.CreateMinerParams{
		Worker:     minerOwnerAddr,
		Peer:       pid,
		SectorSize: sectorSize,
	}

	smsgCid, _, err := plumbing.MessageSend(
		ctx,
		minerOwnerAddr,
		builtin.StorageMarketActorAddr,
		collateral,
		gasPrice,
		gasLimit,
		builtin.MethodsPower.CreateMiner,
		&params,
	)
	if err != nil {
		return nil, err
	}

	var minerAddr address.Address
	err = plumbing.MessageWait(ctx, smsgCid, func(blk *block.Block, smsg *types.SignedMessage, receipt *vm.MessageReceipt) (err error) {
		if receipt.ExitCode != exitcode.Ok {
			// Dragons: do we want to have this back?
			return fmt.Errorf("Error executing actor code (exitcode: %d)", receipt.ExitCode)
		}
		minerAddr, err = address.NewFromBytes(receipt.ReturnValue)
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
	MessagePreview(ctx context.Context, from, to address.Address, method abi.MethodNum, params ...interface{}) (types.GasUnits, error)
	NetworkGetPeerID() peer.ID
	WalletDefaultAddress() (address.Address, error)
}

// MinerPreviewCreate previews the Gas cost of creating a miner
func MinerPreviewCreate(
	ctx context.Context,
	plumbing mpcAPI,
	fromAddr address.Address,
	sectorSize abi.SectorSize,
	pid peer.ID,
) (usedGas types.GasUnits, err error) {
	if fromAddr.Empty() {
		fromAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return types.GasUnits(0), err
		}
	}

	if pid == "" {
		pid = plumbing.NetworkGetPeerID()
	}

	if _, err := plumbing.ConfigGet("mining.minerAddress"); err != nil {
		return types.GasUnits(0), fmt.Errorf("can only have one miner per node")
	}

	usedGas, err = plumbing.MessagePreview(
		ctx,
		fromAddr,
		builtin.StorageMarketActorAddr,
		builtin.MethodsPower.CreateMiner,
		sectorSize,
		pid,
	)
	if err != nil {
		return types.GasUnits(0), errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
	}

	return usedGas, nil
}

// MinerSetPriceResponse collects relevant stats from the set price process
type MinerSetPriceResponse struct {
	MinerAddr address.Address
	Price     types.AttoFIL
}

type minerStatusPlumbing interface {
	MinerStateView(baseKey block.TipSetKey) (MinerStateView, error)
}

// MinerProvingWindow contains a miners proving period start and end as well
// as a set of their proving set.
type MinerProvingWindow struct {
	Start      types.BlockHeight
	End        types.BlockHeight
	ProvingSet map[string]types.Commitments
}

// MinerStatus contains a miners power and the total power of the network
type MinerStatus struct {
	ActorAddress  address.Address
	OwnerAddress  address.Address
	WorkerAddress address.Address
	PeerID        peer.ID
	SectorSize    abi.SectorSize

	Power             abi.StoragePower
	PledgeRequirement abi.TokenAmount
	PledgeBalance     abi.TokenAmount

	ProvingPeriodStart abi.ChainEpoch
	ProvingPeriodEnd   abi.ChainEpoch
	PoStFailureCount   int
	NetworkPower       abi.StoragePower
}

// MinerGetStatus queries the power of a given miner.
func MinerGetStatus(ctx context.Context, plumbing minerStatusPlumbing, minerAddr address.Address, key block.TipSetKey) (MinerStatus, error) {
	view, err := plumbing.MinerStateView(key)
	if err != nil {
		return MinerStatus{}, err
	}
	owner, worker, err := view.MinerControlAddresses(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	peerID, err := view.MinerPeerID(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	sectorSize, err := view.MinerSectorSize(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	periodStart, periodEnd, failureCount, err := view.MinerProvingPeriod(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	claimedPower, err := view.MinerClaimedPower(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	totalPower, err := view.NetworkTotalPower(ctx)
	if err != nil {
		return MinerStatus{}, err
	}
	requirement, balance, err := view.MinerPledgeCollateral(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}

	return MinerStatus{
		ActorAddress:  minerAddr,
		OwnerAddress:  owner,
		WorkerAddress: worker,
		PeerID:        peerID,
		SectorSize:    sectorSize,

		Power:             claimedPower,
		PledgeRequirement: requirement,
		PledgeBalance:     balance,

		ProvingPeriodStart: periodStart,
		ProvingPeriodEnd:   periodEnd,
		PoStFailureCount:   failureCount,
		NetworkPower:       totalPower,
	}, nil
}

// mwapi is the subset of the plumbing.API that MinerSetWorkerAddress use.
type mwapi interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ChainHeadKey() block.TipSetKey
	MinerStateView(baseKey block.TipSetKey) (MinerStateView, error)
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasPrice types.AttoFIL, gasLimit types.GasUnits, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error)
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

	head := plumbing.ChainHeadKey()
	state, err := plumbing.MinerStateView(head)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not get miner owner address")
	}

	owner, _, err := state.MinerControlAddresses(ctx, minerAddr)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "could not get miner owner address")
	}

	c, _, err := plumbing.MessageSend(
		ctx,
		owner,
		minerAddr,
		types.ZeroAttoFIL,
		gasPrice,
		gasLimit,
		builtin.MethodsMiner.ChangeWorkerAddress,
		&workerAddr)
	return c, err
}
