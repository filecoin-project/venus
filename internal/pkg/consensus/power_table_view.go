package consensus

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	sector "github.com/filecoin-project/go-sectorbuilder"
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
	rets, err := v.snapshot.Query(ctx, address.Undef, address.StoragePowerAddress, power.GetTotalPower)
	if err != nil {
		return nil, err
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// Miner returns the storage that this miner has committed to the network.
func (v PowerTableView) Miner(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error) {
	rets, err := v.snapshot.Query(ctx, address.Undef, address.StoragePowerAddress, power.GetPowerReport, mAddr)
	if err != nil {
		return nil, err
	}
	if len(rets) == 0 {
		return nil, errors.Errorf("invalid nil return value from GetPowerReport")
	}

	reportValue, err := abi.Deserialize(rets[0], abi.PowerReport)
	if err != nil {
		return nil, err
	}
	r, ok := reportValue.Val.(types.PowerReport)
	if !ok {
		return nil, errors.Errorf("invalid report bytes returned from GetPower")
	}

	return r.ActivePower, nil
}

// WorkerAddr returns the address of the miner worker given the miner address.
func (v PowerTableView) WorkerAddr(ctx context.Context, mAddr address.Address) (address.Address, error) {
	rets, err := v.snapshot.Query(ctx, address.Undef, mAddr, miner.GetWorker)
	if err != nil {
		return address.Undef, err
	}

	if len(rets) == 0 {
		return address.Undef, errors.Errorf("invalid nil return value from GetWorker")
	}

	addrValue, err := abi.Deserialize(rets[0], abi.Address)
	if err != nil {
		return address.Undef, err
	}
	a, ok := addrValue.Val.(address.Address)
	if !ok {
		return address.Undef, errors.Errorf("invalid address bytes returned from GetWorker")
	}
	return a, nil
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
func (v PowerTableView) SortedSectorInfos(ctx context.Context, mAddr address.Address) (sector.SortedSectorInfo, error) {
	// Dragons: once we have a real VM we must get the sector infos from the
	// miner actor.  For now we return a fake constant.
	var fakeCommR1, fakeCommR2 [sector.CommitmentBytesLen]byte
	fakeCommR1[0], fakeCommR1[1] = 0xa, 0xb
	fakeCommR2[0], fakeCommR2[1] = 0xc, 0xd
	sectorID1, sectorID2 := uint64(0), uint64(1)

	psi1 := sector.SectorInfo{
		SectorID: sectorID1,
		CommR:    fakeCommR1,
	}
	psi2 := sector.SectorInfo{
		SectorID: sectorID2,
		CommR:    fakeCommR2,
	}

	return sector.NewSortedSectorInfo(psi1, psi2), nil
}
