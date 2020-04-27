package consensus

import (
	"context"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// PowerStateView is the consensus package's interface to chain state.
type PowerStateView interface {
	state.AccountStateView
	MinerSectorSize(ctx context.Context, maddr addr.Address) (abi.SectorSize, error)
	MinerControlAddresses(ctx context.Context, maddr addr.Address) (owner, worker addr.Address, err error)
	MinerSectorsForEach(ctx context.Context, maddr addr.Address, f func(id abi.SectorNumber, sealedCID cid.Cid, rpp abi.RegisteredProof, dealIDs []abi.DealID) error) error
	NetworkTotalRawBytePower(ctx context.Context) (abi.StoragePower, error)
	MinerClaimedRawBytePower(ctx context.Context, miner addr.Address) (abi.StoragePower, error)
}

// FaultStateView is the interface needed by the recent fault state to adjust
// miner power claims based on consensus slashing events.
type FaultStateView interface {
	MinerExists(ctx context.Context, maddr addr.Address) (bool, error)
}

// PowerTableView defines the set of functions used by the ChainManager to view
// the power table encoded in the tipset's state tree
// PowerTableView is the power table view used for running expected consensus in
type PowerTableView struct {
	state      PowerStateView
	faultState FaultStateView
}

// NewPowerTableView constructs a new view with a snapshot pinned to a particular tip set.
func NewPowerTableView(state PowerStateView, faultState FaultStateView) PowerTableView {
	return PowerTableView{
		state:      state,
		faultState: faultState,
	}
}

// Total returns the total storage as a BytesAmount.
func (v PowerTableView) Total(ctx context.Context) (abi.StoragePower, error) {
	return v.state.NetworkTotalRawBytePower(ctx)
}

// MinerClaim returns the storage that this miner claims to have committed to the network.
func (v PowerTableView) MinerClaim(ctx context.Context, mAddr addr.Address) (abi.StoragePower, error) {
	claim, err := v.state.MinerClaimedRawBytePower(ctx, mAddr)
	if err != nil {
		return abi.NewStoragePower(0), err
	}
	// Only return claim if fault state still tracks miner
	exists, err := v.faultState.MinerExists(ctx, mAddr)
	if err != nil {
		return abi.NewStoragePower(0), err
	}
	if !exists { // miner was slashed
		return abi.NewStoragePower(0), nil
	}

	return claim, nil
}

// WorkerAddr returns the address of the miner worker given the miner address.
func (v PowerTableView) WorkerAddr(ctx context.Context, mAddr addr.Address) (addr.Address, error) {
	_, worker, err := v.state.MinerControlAddresses(ctx, mAddr)
	return worker, err
}

// SignerAddress returns the public key address associated with the given address.
func (v PowerTableView) SignerAddress(ctx context.Context, a addr.Address) (addr.Address, error) {
	return v.state.AccountSignerAddress(ctx, a)
}

// HasClaimedPower returns true if the provided address belongs to a miner with claimed power in the storage market
func (v PowerTableView) HasClaimedPower(ctx context.Context, mAddr addr.Address) (bool, error) {
	numBytes, err := v.MinerClaim(ctx, mAddr)
	if err == types.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return numBytes.GreaterThan(big.Zero()), nil
}

// SortedSectorInfos returns the sector information for the given miner
func (v PowerTableView) SortedSectorInfos(ctx context.Context, mAddr addr.Address) ([]abi.SectorInfo, error) {
	var infos []abi.SectorInfo
	err := v.state.MinerSectorsForEach(ctx, mAddr, func(id abi.SectorNumber, sealedCID cid.Cid, rpp abi.RegisteredProof, _ []abi.DealID) error {
		infos = append(infos, abi.SectorInfo{
			SectorNumber:    id,
			SealedCID:       sealedCID,
			RegisteredProof: rpp,
		})
		return nil
	})
	return infos, err
}
