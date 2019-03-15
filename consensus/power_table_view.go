package consensus

import (
	"context"
	"math/big"

	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/vm"
)

// PowerTableView defines the set of functions used by the ChainManager to view
// the power table encoded in the tipset's state tree
type PowerTableView interface {
	// Total returns the total bytes stored by all miners in the given
	// state.
	Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error)

	// Miner returns the total bytes stored by the miner of the
	// input address in the given state.
	Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (uint64, error)

	// HasPower returns true if the input address is associated with a
	// miner that has storage power in the network.
	HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool
}

// MarketView is the power table view used for running expected consensus in
// production.  It's methods use data from an input state's storage market to
// determine power values in a chain.
type MarketView struct{}

var _ PowerTableView = &MarketView{}

// Total returns the total storage as a uint64.  If the total storage
// value exceeds the max value of a uint64 this method errors.
// TODO: uint64 has enough bits to express about 1 exabyte of total storage.
// This should be increased for v1.
func (v *MarketView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error) {
	vms := vm.NewStorageMap(bstore)
	rets, ec, err := CallQueryMethod(ctx, st, vms, address.StorageMarketAddress, "getTotalStorage", []byte{}, address.Undef, nil)
	if err != nil {
		return 0, err
	}

	if ec != 0 {
		return 0, errors.Errorf("non-zero return code from query message: %d", ec)
	}
	res := big.NewInt(0)
	res.SetBytes(rets[0])

	return res.Uint64(), nil
}

// Miner returns the storage that this miner has committed as a uint64.
// If the total storage value exceeds the max value of a uint64 this method
// errors.
// TODO: currently power is in sectors, figure out if & how it should be converted to bytes.
// TODO: uint64 has enough bits to express about 1 exabyte.  This
// should probably be increased for v1.
func (v *MarketView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (uint64, error) {
	vms := vm.NewStorageMap(bstore)
	rets, ec, err := CallQueryMethod(ctx, st, vms, mAddr, "getPower", []byte{}, address.Undef, nil)
	if err != nil {
		return 0, err
	}

	if ec != 0 {
		return 0, errors.Errorf("non-zero return code from query message: %d", ec)
	}
	ret := big.NewInt(0).SetBytes(rets[0])

	return ret.Uint64(), nil
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

	return numBytes > 0
}
