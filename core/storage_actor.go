package core

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(StorageMarketStorage{})
	cbor.RegisterCborType(struct{}{})
}

type StorageMarketActor struct{}

type StorageMarketStorage struct {
	Miners map[types.Address]struct{}
}

var _ ExecutableActor = (*StorageMarketActor)(nil)

func NewStorageMarketActor() (*types.Actor, error) {
	storageBytes, err := MarshalStorage(&StorageMarketStorage{make(map[types.Address]struct{})})
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.StorageMarketActorCodeCid, nil, storageBytes), nil
}

func (sma *StorageMarketActor) Exports() Exports {
	return storageMarketExports
}

var storageMarketExports = Exports{
	"createMiner": &FunctionSignature{
		Params: []interface{}{&big.Int{}},
		Return: types.Address(""),
	},
}

func (sma *StorageMarketActor) CreateMiner(ctx *VMContext, pledge *big.Int) (types.Address, uint8, error) {
	var storage StorageMarketStorage
	if err := cbor.DecodeInto(ctx.ReadStorage(), &storage); err != nil {
		return "", 1, errors.Wrapf(err, "failed to load storage market actors data")
	}

	addr := ctx.AddressForNewActor()

	miner, err := NewMinerActor(ctx.message.From, pledge, ctx.message.Value)
	if err != nil {
		return "", 1, err
	}

	if err := transfer(ctx.from, miner, ctx.message.Value); err != nil {
		return "", 1, err
	}

	if err := ctx.state.SetActor(context.TODO(), addr, miner); err != nil {
		return "", 1, err
	}

	storage.Miners[addr] = struct{}{}

	data, err := cbor.DumpObject(storage)
	if err != nil {
		return "", 1, err
	}

	if err := ctx.WriteStorage(data); err != nil {
		return "", 1, err
	}

	return addr, 0, nil
}
