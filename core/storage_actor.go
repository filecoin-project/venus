package core

import (
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(StorageMarketStorage{})
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
		Return: nil,
	},
}
