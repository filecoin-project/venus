package consensus

import (
	"context"
	"fmt"

	ffi "github.com/filecoin-project/filecoin-ffi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// PowerTableView defines the set of functions used by the ChainManager to view
// the power table encoded in the tipset's state tree
// PowerTableView is the power table view used for running expected consensus in
type PowerTableView struct {
	snapshot ActorStateSnapshot
}

// NewPowerTableView constructs a new view with a snapshot pinned to a particular tip set.
func NewPowerTableView(q ActorStateSnapshot) PowerTableView {
	return PowerTableView{
		snapshot: q,
	}
}

// Total returns the total storage as a BytesAmount.
func (v PowerTableView) Total(ctx context.Context) (*types.BytesAmount, error) {
	// Dragons: re-write without using query

	// rets, err := v.snapshot.Query(ctx, address.Undef, address.StoragePowerAddress, power.GetTotalPower)
	// if err != nil {
	// 	return nil, err
	// }

	// return types.NewBytesAmountFromBytes(rets[0]), nil
	return types.NewBytesAmount(0), nil
}

// Miner returns the storage that this miner has committed to the network.
func (v PowerTableView) Miner(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error) {
	// Dragons: re-write without using query

	// rets, err := v.snapshot.Query(ctx, address.Undef, address.StoragePowerAddress, power.GetPowerReport, mAddr)
	// if err != nil {
	// 	return nil, err
	// }
	// if len(rets) == 0 {
	// 	return nil, errors.Errorf("invalid nil return value from GetPowerReport")
	// }

	// reportValue, err := abi.Deserialize(rets[0], abi.PowerReport)
	// if err != nil {
	// 	return nil, err
	// }
	// r, ok := reportValue.Val.(types.PowerReport)
	// if !ok {
	// 	return nil, errors.Errorf("invalid report bytes returned from GetPower")
	// }

	// return r.ActivePower, nil

	return types.NewBytesAmount(0), nil
}

// WorkerAddr returns the address of the miner worker given the miner address.
func (v PowerTableView) WorkerAddr(ctx context.Context, mAddr address.Address) (address.Address, error) {
	// Dragons: re-write without using query

	// rets, err := v.snapshot.Query(ctx, address.Undef, mAddr, miner.GetWorker)
	// if err != nil {
	// 	return address.Undef, err
	// }

	// if len(rets) == 0 {
	// 	return address.Undef, errors.Errorf("invalid nil return value from GetWorker")
	// }

	// addrValue, err := abi.Deserialize(rets[0], abi.Address)
	// if err != nil {
	// 	return address.Undef, err
	// }
	// a, ok := addrValue.Val.(address.Address)
	// if !ok {
	// 	return address.Undef, errors.Errorf("invalid address bytes returned from GetWorker")
	// }
	// return a, nil
	return address.Undef, nil
}

// HasPower returns true if the provided address belongs to a miner with power
// in the storage market
func (v PowerTableView) HasPower(ctx context.Context, mAddr address.Address) (bool, error) {
	numBytes, err := v.Miner(ctx, mAddr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return numBytes.GreaterThan(types.ZeroBytes), nil
}

// SortedSectorInfos returns the sector information for the given miner
func (v PowerTableView) SortedSectorInfos(ctx context.Context, mAddr address.Address) (ffi.SortedPublicSectorInfo, error) {
	// Dragons: once we have a real VM we must get the sector infos from the
	// miner actor.  For now we return a set of fake sectors matching the
	// total power reported on chain.

	var fakeSectors ffi.SortedPublicSectorInfo
	numSectors, err := v.NumSectors(ctx, mAddr)
	if err != nil {
		return fakeSectors, nil
	}

	fakeSectors = NFakeSectorInfos(numSectors)
	return fakeSectors, nil
}

// SectorSize returns the sector size for this miner
func (v PowerTableView) SectorSize(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error) {
	// Dragons: re-write without using query

	// rets, err := v.snapshot.Query(ctx, address.Undef, address.StoragePowerAddress, power.GetSectorSize, mAddr)
	// if err != nil {
	// 	return nil, err
	// }

	// if len(rets) == 0 {
	// 	return nil, errors.Errorf("invalid nil return value from GetSectorSize")
	// }

	// sectorSizeValue, err := abi.Deserialize(rets[0], abi.BytesAmount)
	// if err != nil {
	// 	return nil, err
	// }
	// ss, ok := sectorSizeValue.Val.(*types.BytesAmount)
	// if !ok {
	// 	return nil, errors.Errorf("invalid sectorsize bytes returned from GetWorker")
	// }
	// return ss, nil

	return types.NewBytesAmount(0), nil
}

// NumSectors returns the number of sectors this miner has committed
func (v PowerTableView) NumSectors(ctx context.Context, mAddr address.Address) (uint64, error) {
	minerBytes, err := v.Miner(ctx, mAddr)
	if err != nil {
		return 0, err
	}
	sectorSize, err := v.SectorSize(ctx, mAddr)
	if err != nil {
		return 0, err
	}
	if minerBytes.Uint64()%sectorSize.Uint64() != 0 {
		return 0, fmt.Errorf("total power byte count %d is not a multiple of sector size %d ", minerBytes.Uint64(), sectorSize.Uint64())
	}
	return minerBytes.Uint64() / sectorSize.Uint64(), nil
}
