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
type PowerTableView interface {
	// Total returns the total bytes stored by all miners in the given
	// state.
	Total(ctx context.Context) (*types.BytesAmount, error)

	// Miner returns the total bytes stored by the miner of the
	// input address in the given state.
	Miner(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error)

	// HasPower returns true if the input address is associated with a
	// miner that has storage power in the network.
	HasPower(ctx context.Context, mAddr address.Address) bool

	// WorkerAddr returns the address of the miner worker given the miner addr
	WorkerAddr(context.Context, address.Address) (address.Address, error)
}

// MarketView is the power table view used for running expected consensus in
// production.  It's methods use data from an input state's storage market to
// determine power values in a chain.
type MarketView struct {
	queryer ActorStateQueryer
}

var _ PowerTableView = (*MarketView)(nil)

func NewMarketView(q ActorStateQueryer) *MarketView {
	return &MarketView{
		queryer: q,
	}
}

// Total returns the total storage as a BytesAmount.
func (v *MarketView) Total(ctx context.Context) (*types.BytesAmount, error) {
	rets, err := v.queryer.Query(ctx, address.Undef, address.StorageMarketAddress, "getTotalStorage")
	if err != nil {
		return nil, err
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// Miner returns the storage that this miner has committed to the network.
func (v *MarketView) Miner(ctx context.Context, mAddr address.Address) (*types.BytesAmount, error) {
	rets, err := v.queryer.Query(ctx, address.Undef, mAddr, "getPower")
	if err != nil {
		return nil, err
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// WorkerAddr returns the address of the miner worker given the miner address.
func (v *MarketView) WorkerAddr(ctx context.Context, mAddr address.Address) (address.Address, error) {
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
func (v *MarketView) HasPower(ctx context.Context, mAddr address.Address) bool {
	numBytes, err := v.Miner(ctx, mAddr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			return false
		}

		panic(err) //hey guys, dropping errors is BAD
	}

	return numBytes.GreaterThan(types.ZeroBytes)
}
