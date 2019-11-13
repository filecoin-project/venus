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
	rets, err := v.snapshot.Query(ctx, address.Undef, address.PowerAddress, power.GetTotalPower)
	if err != nil {
		return nil, err
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// Miner returns the storage that this miner has committed to the network.
func (v PowerTableView) Miner(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error) {
	rets, err := v.snapshot.Query(ctx, address.Undef, address.PowerAddress, power.GetPowerReport, mAddr)
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
func (v PowerTableView) HasPower(ctx context.Context, mAddr address.Address) bool {
	numBytes, err := v.Miner(ctx, mAddr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			return false
		}

		panic(err) //hey guys, dropping errors is BAD
	}

	return numBytes.GreaterThan(types.ZeroBytes)
}
