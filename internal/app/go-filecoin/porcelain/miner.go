package porcelain

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/power"
	cid "github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// mcAPI is the subset of the plumbing.API that MinerCreate uses.
type mcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasBaseFee, gasPremium types.AttoFIL, gasLimit gas.Unit, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error)
	MessageWait(ctx context.Context, msgCid cid.Cid, lookback uint64, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error
	WalletDefaultAddress() (address.Address, error)
}

type MinerStateView interface {
	MinerControlAddresses(ctx context.Context, maddr address.Address) (owner, worker address.Address, err error)
	MinerPeerID(ctx context.Context, maddr address.Address) (peer.ID, error)
	MinerSectorConfiguration(ctx context.Context, maddr address.Address) (*state.MinerSectorConfiguration, error)
	MinerSectorCount(ctx context.Context, maddr address.Address) (uint64, error)
	PowerNetworkTotal(ctx context.Context) (*state.NetworkPower, error)
	MinerClaimedPower(ctx context.Context, miner address.Address) (raw, qa abi.StoragePower, err error)
	MinerInfo(ctx context.Context, maddr address.Address) (*miner.MinerInfo, error)
}

// MinerCreate creates a new miner actor for the given account and returns its address.
// It will wait for the the actor to appear on-chain and add set the address to mining.minerAddress in the config.
// TODO: add ability to pass in a KeyInfo to store for signing blocks.
//       See https://github.com/filecoin-project/go-filecoin/issues/1843
func MinerCreate(
	ctx context.Context,
	plumbing mcAPI,
	minerOwnerAddr address.Address,
	gasBaseFee types.AttoFIL,
	gasPremium types.AttoFIL,
	gasLimit gas.Unit,
	sealProofType abi.RegisteredSealProof,
	pid peer.ID,
	collateral types.AttoFIL,
) (_ address.Address, err error) {
	if minerOwnerAddr == (address.Address{}) {
		minerOwnerAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return address.Undef, err
		}
	}

	addr, err := plumbing.ConfigGet("mining.minerAddress")
	if err != nil {
		return address.Undef, err
	}
	if addr != address.Undef {
		return address.Undef, fmt.Errorf("can only have one miner per node")
	}

	params := power.CreateMinerParams{
		Worker:        minerOwnerAddr,
		Owner:         minerOwnerAddr,
		Peer:          abi.PeerID(pid),
		SealProofType: sealProofType,
	}

	smsgCid, _, err := plumbing.MessageSend(
		ctx,
		minerOwnerAddr,
		builtin.StoragePowerActorAddr,
		collateral,
		gasBaseFee,
		gasPremium,
		gasLimit,
		builtin.MethodsPower.CreateMiner,
		&params,
	)
	if err != nil {
		return address.Undef, err
	}

	var result power.CreateMinerReturn
	err = plumbing.MessageWait(ctx, smsgCid, msg.DefaultMessageWaitLookback, func(blk *block.Block, smsg *types.SignedMessage, receipt *types.MessageReceipt) (err error) {
		if receipt.ExitCode != exitcode.Ok {
			// Dragons: do we want to have this back?
			return fmt.Errorf("Error executing actor code (exitcode: %d)", receipt.ExitCode)
		}
		return encoding.Decode(receipt.ReturnValue, &result)
	})
	if err != nil {
		return address.Undef, err
	}

	if err = plumbing.ConfigSet("mining.minerAddress", result.RobustAddress.String()); err != nil {
		return address.Undef, err
	}

	return result.RobustAddress, nil
}

// mpcAPI is the subset of the plumbing.API that MinerPreviewCreate uses.
type mpcAPI interface {
	ConfigGet(dottedPath string) (interface{}, error)
	MessagePreview(ctx context.Context, from, to address.Address, method abi.MethodNum, params ...interface{}) (gas.Unit, error)
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
) (usedGas gas.Unit, err error) {
	if fromAddr.Empty() {
		fromAddr, err = plumbing.WalletDefaultAddress()
		if err != nil {
			return gas.NewGas(0), err
		}
	}

	if pid == "" {
		pid = plumbing.NetworkGetPeerID()
	}

	if _, err := plumbing.ConfigGet("mining.minerAddress"); err != nil {
		return gas.NewGas(0), fmt.Errorf("can only have one miner per node")
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
		return gas.NewGas(0), errors.Wrap(err, "Could not create miner. Please consult the documentation to setup your wallet and genesis block correctly")
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
	ChainTipSet(key block.TipSetKey) (*block.TipSet, error)
}

// MinerProvingWindow contains a miners proving period start and end as well
// as a set of their proving set.
type MinerProvingWindow struct {
	Start      abi.ChainEpoch
	End        abi.ChainEpoch
	ProvingSet map[string]types.Commitments
}

// MinerStatus contains a miners power and the total power of the network
type MinerStatus struct {
	ActorAddress  address.Address
	OwnerAddress  address.Address
	WorkerAddress address.Address
	PeerID        peer.ID

	SealProofType              abi.RegisteredSealProof
	SectorSize                 abi.SectorSize
	WindowPoStPartitionSectors uint64
	SectorCount                uint64
	PoStFailureCount           int

	RawPower                    abi.StoragePower
	NetworkRawPower             abi.StoragePower
	NetworkQualityAdjustedPower abi.StoragePower
	QualityAdjustedPower        abi.StoragePower
}

// MinerGetStatus queries the power of a given miner.
func MinerGetStatus(ctx context.Context, plumbing minerStatusPlumbing, minerAddr address.Address, key block.TipSetKey) (MinerStatus, error) {
	view, err := plumbing.MinerStateView(key)
	if err != nil {
		return MinerStatus{}, err
	}
	sectorCount, err := view.MinerSectorCount(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	minerInfo, err := view.MinerInfo(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	rawPower, qaPower, err := view.MinerClaimedPower(ctx, minerAddr)
	if err != nil {
		return MinerStatus{}, err
	}
	totalPower, err := view.PowerNetworkTotal(ctx)
	if err != nil {
		return MinerStatus{}, err
	}

	return MinerStatus{
		ActorAddress:  minerAddr,
		OwnerAddress:  minerInfo.Owner,
		WorkerAddress: minerInfo.Worker,
		PeerID:        *minerInfo.PeerId,

		SealProofType:              minerInfo.SealProofType,
		SectorSize:                 minerInfo.SectorSize,
		WindowPoStPartitionSectors: minerInfo.WindowPoStPartitionSectors,
		SectorCount:                sectorCount,

		RawPower:                    rawPower,
		QualityAdjustedPower:        qaPower,
		NetworkRawPower:             totalPower.RawBytePower,
		NetworkQualityAdjustedPower: totalPower.QualityAdjustedPower,
	}, nil
}

// mwapi is the subset of the plumbing.API that MinerSetWorkerAddress use.
type mwapi interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ChainHeadKey() block.TipSetKey
	MinerStateView(baseKey block.TipSetKey) (MinerStateView, error)
	MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, gasBaseFee, gasPremium types.AttoFIL, gasLimit gas.Unit, method abi.MethodNum, params interface{}) (cid.Cid, chan error, error)
}

// MinerSetWorkerAddress sets the worker address of the miner actor to the provided new address,
// waits for the message to appear on chain and then sets miner.workerAddr config to the new address.
func MinerSetWorkerAddress(
	ctx context.Context,
	plumbing mwapi,
	workerAddr address.Address,
	gasBaseFee, gasPremium types.AttoFIL,
	gasLimit gas.Unit,
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
		gasBaseFee,
		gasPremium,
		gasLimit,
		builtin.MethodsMiner.ChangeWorkerAddress,
		&workerAddr)
	return c, err
}
