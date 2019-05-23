package consensus

import (
	"context"

	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// PowerTableView defines the set of functions used by the ChainManager to view
// the power table encoded in the tipset's state tree
type PowerTableView interface {
	// Total returns the total bytes stored by all miners in the given
	// state.
	Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error)

	// Miner returns the total bytes stored by the miner of the
	// input address in the given state.
	Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error)

	// HasPower returns true if the input address is associated with a
	// miner that has storage power in the network.
	HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool
}

// MarketView is the power table view used for running expected consensus in
// production.  It's methods use data from an input state's storage market to
// determine power values in a chain.
type MarketView struct{}

var _ PowerTableView = &MarketView{}

// Total returns the total storage as a BytesAmount.
func (v *MarketView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error) {
	vms := vm.NewStorageMap(bstore)
	rets, ec, err := CallQueryMethod(ctx, st, vms, address.StorageMarketAddress, "getTotalStorage", []byte{}, address.Undef, nil)
	if err != nil {
		return nil, err
	}

	if ec != 0 {
		return nil, errors.Errorf("non-zero return code from query message: %d", ec)
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// Miner returns the storage that this miner has committed to the network.
func (v *MarketView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	vms := vm.NewStorageMap(bstore)
	rets, ec, err := CallQueryMethod(ctx, st, vms, mAddr, "getPower", []byte{}, address.Undef, nil)
	if err != nil {
		return nil, err
	}

	if ec != 0 {
		return nil, errors.Errorf("non-zero return code from query message: %d", ec)
	}

	return types.NewBytesAmountFromBytes(rets[0]), nil
}

// HasPower returns true if the provided address belongs to a miner with power
// in the storage market
func (v *MarketView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	numBytes, err := v.Miner(ctx, st, bstore, mAddr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			return false
		}

		panic(err) //hey guys, dropping errors is BAD
	}

	return numBytes.GreaterThan(types.ZeroBytes)
}
