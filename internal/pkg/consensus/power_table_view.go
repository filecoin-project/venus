package consensus

import (
	"context"
	"fmt"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/ipfs/go-cid"
)

// PowerStateView is the consensus package's interface to chain state.
type PowerStateView interface {
	AccountStateView
	MinerSectorSize(ctx context.Context, maddr addr.Address) (abi.SectorSize, error)
	MinerControlAddresses(ctx context.Context, maddr addr.Address) (owner, worker addr.Address, err error)
	MinerProvingSetForEach(ctx context.Context, maddr addr.Address, f func(id abi.SectorNumber, sealedCID cid.Cid, rpp abi.RegisteredProof) error) error
	NetworkTotalPower(ctx context.Context) (abi.StoragePower, error)
	MinerClaimedPower(ctx context.Context, miner addr.Address) (abi.StoragePower, error)
}

// PowerTableView defines the set of functions used by the ChainManager to view
// the power table encoded in the tipset's state tree
// PowerTableView is the power table view used for running expected consensus in
type PowerTableView struct {
	state PowerStateView
}

// NewPowerTableView constructs a new view with a snapshot pinned to a particular tip set.
func NewPowerTableView(state PowerStateView) PowerTableView {
	return PowerTableView{state}
}

// Total returns the total storage as a BytesAmount.
func (v PowerTableView) Total(ctx context.Context) (abi.StoragePower, error) {
	return v.state.NetworkTotalPower(ctx)
}

// MinerClaim returns the storage that this miner claims to have committed to the network.
func (v PowerTableView) MinerClaim(ctx context.Context, mAddr addr.Address) (abi.StoragePower, error) {
	return v.state.MinerClaimedPower(ctx, mAddr)
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
	err := v.state.MinerProvingSetForEach(ctx, mAddr, func(id abi.SectorNumber, sealedCID cid.Cid, rpp abi.RegisteredProof) error {
		infos = append(infos, abi.SectorInfo{
			SectorNumber:    id,
			SealedCID:       sealedCID,
			RegisteredProof: rpp,
		})
		return nil
	})
	return infos, err
}

// SectorSize returns the sector size for this miner
func (v PowerTableView) SectorSize(ctx context.Context, mAddr addr.Address) (abi.SectorSize, error) {
	return v.state.MinerSectorSize(ctx, mAddr)
}

// NumSectors returns the number of sectors this miner has committed, computed as the quotient of the miner's claimed
// power and sector size.
func (v PowerTableView) NumSectors(ctx context.Context, mAddr addr.Address) (uint64, error) {
	minerBytes, err := v.MinerClaim(ctx, mAddr)
	if err != nil {
		return 0, err
	}
	sectorSize, err := v.SectorSize(ctx, mAddr)
	if err != nil {
		return 0, err
	}
	if minerBytes.Uint64()%uint64(sectorSize) != 0 {
		return 0, fmt.Errorf("total power byte count %d is not a multiple of sector size %d ", minerBytes.Uint64(), sectorSize)
	}
	return minerBytes.Uint64() / uint64(sectorSize), nil
}
