package core

import (
	"context"

	"gx/ipfs/QmSKyB5faguXT4NqbrXpnRXqaVj5DhSm7x9BtzFydBY1UK/go-leb128"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

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
	Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error)

	// Miner returns the total bytes stored by the miner of the
	// input address in the given state.
	Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr types.Address) (uint64, error)

	// HasPower returns true if the input address is associated with a
	// miner that has storage power in the network.
	HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr types.Address) bool
}

type marketView struct{}

var _ PowerTableView = &marketView{}

// Total returns the total storage as a uint64.  If the total storage
// value exceeds the max value of a uint64 this method errors.
// TODO: uint64 has enough bits to express about 1 exabyte of total storage.
// This should be increased for v1.
func (v *marketView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error) {
	vms := vm.NewStorageMap(bstore)
	rets, ec, err := CallQueryMethod(ctx, st, vms, address.StorageMarketAddress, "getTotalStorage", []byte{}, types.Address{}, nil)
	if err != nil {
		return 0, err
	}

	if ec != 0 {
		return 0, errors.Errorf("Non-zero return code from query message: %d", ec)
	}

	return leb128.ToUInt64(rets[0]), nil
}

// Miner returns the storage that this miner has committed as a uint64.
// If the total storage value exceeds the max value of a uint64 this method
// errors. TODO: uint64 has enough bits to express about 1 exabyte.  This
// should probably be increased for v1.
func (v *marketView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr types.Address) (uint64, error) {
	vms := vm.NewStorageMap(bstore)
	rets, ec, err := CallQueryMethod(ctx, st, vms, mAddr, "getStorage", []byte{}, types.Address{}, nil)
	if err != nil {
		return 0, err
	}

	if ec != 0 {
		return 0, errors.Errorf("Non-zero return code from query message: %d", ec)
	}

	return leb128.ToUInt64(rets[0]), nil
}

// HasPower returns true if the provided address belongs to a miner with power
// in the storage market
func (v *marketView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr types.Address) bool {
	numBytes, err := v.Miner(ctx, st, bstore, mAddr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			return false
		}

		panic(err) //hey guys, dropping errors is BAD
	}

	return numBytes > 0
}
