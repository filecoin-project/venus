package consensus

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// PowerTableView defines the set of functions used by the ChainManager to view
// the power table encoded in the tipset's state tree
// PowerTableView is the power table view used for running expected consensus in
type PowerTableView struct {
	queryer ActorStateQueryer
}

// NewPowerTableView constructs a new view with a queryer pinned to a particular tip set.
func NewPowerTableView(q ActorStateQueryer) PowerTableView {
	return PowerTableView{
		queryer: q,
	}
}

// Total returns the total storage as a BytesAmount.
func (v PowerTableView) Total(ctx context.Context) (*types.BytesAmount, error) {
	rets, err := v.queryer.Query(ctx, address.Undef, address.StorageMarketAddress, "getTotalStorage")
	if err != nil {
		return nil, err
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// Miner returns the storage that this miner has committed to the network.
func (v PowerTableView) Miner(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error) {
	rets, err := v.queryer.Query(ctx, address.Undef, mAddr, "getPower")
	if err != nil {
		return nil, err
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// WorkerAddr returns the address of the miner worker given the miner address.
func (v PowerTableView) WorkerAddr(ctx context.Context, mAddr address.Address) (address.Address, error) {
	rets, err := v.queryer.Query(ctx, address.Undef, mAddr, "getWorker")
	if err != nil {
		return address.Undef, err
	}

	if len(rets) == 0 {
		return address.Undef, errors.Errorf("invalid nil return value from getWorker")
	}

	addrValue, err := abi.Deserialize(rets[0], abi.Address)
	if err != nil {
		return address.Undef, err
	}
	a, ok := addrValue.Val.(address.Address)
	if !ok {
		return address.Undef, errors.Errorf("invalid address bytes returned from getWorker")
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
