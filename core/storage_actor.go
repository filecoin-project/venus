package core

import (
	"context"
	"fmt"
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
)

// MinimumPledge is the minimum amount of space a user can pledge
var MinimumPledge = big.NewInt(10000)

// ErrPledgeTooLow is returned when the pledge is too low
var ErrPledgeTooLow = &revertErrorWrap{fmt.Errorf("pledge must be at least %s bytes", MinimumPledge)}

func init() {
	cbor.RegisterCborType(StorageMarketStorage{})
	cbor.RegisterCborType(struct{}{})
}

// StorageMarketActor implements the filecoin storage market. It is responsible
// for starting up new miners, adding bids, asks and deals. It also exposes the
// power table used to drive filecoin consensus.
type StorageMarketActor struct{}

// StorageMarketStorage is the storage markets storage
type StorageMarketStorage struct {
	Miners map[types.Address]struct{}
}

var _ ExecutableActor = (*StorageMarketActor)(nil)

// NewStorageMarketActor returns a new storage market actor
func NewStorageMarketActor() (*types.Actor, error) {
	storageBytes, err := MarshalStorage(&StorageMarketStorage{make(map[types.Address]struct{})})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.StorageMarketActorCodeCid, nil, storageBytes), nil
}

// Exports returns the actors exports
func (sma *StorageMarketActor) Exports() Exports {
	return storageMarketExports
}

var storageMarketExports = Exports{
	"createMiner": &FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: []abi.Type{abi.Address},
	},
}

// CreateMiner creates a new miner with the a pledge of the given size. The
// miners collateral is set by the value in the message.
func (sma *StorageMarketActor) CreateMiner(ctx *VMContext, pledge *big.Int) (types.Address, uint8, error) {
	var storage StorageMarketStorage
	ret, err := WithStorage(ctx, &storage, func() (interface{}, error) {
		if pledge.Cmp(MinimumPledge) < 0 {
			return nil, ErrPledgeTooLow
		}

		// 'CreateNewActor' (should likely be a method on the vmcontext)
		addr := ctx.AddressForNewActor()

		miner, err := NewMinerActor(ctx.message.From, pledge, ctx.message.Value)
		if err != nil {
			return nil, err
		}

		if err := ctx.state.SetActor(context.TODO(), addr, miner); err != nil {
			return nil, err
		}
		// -- end --

		_, _, err = ctx.Send(addr, "", ctx.message.Value, nil)
		if err != nil {
			return nil, err
		}

		storage.Miners[addr] = struct{}{}
		return addr, nil
	})
	if err != nil {
		return "", 1, err
	}

	return ret.(types.Address), 0, nil
}
