package core

import (
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(MinerStorage{})
}

type MinerActor struct{}

type MinerStorage struct {
	Owner types.Address

	// Pledge is amount the space being offered up by this miner
	// TODO: maybe minimum granularity is more than 1 byte?
	PledgeBytes *big.Int

	// Collateral is the total amount of filecoin being held as collateral for
	// the miners pledge
	Collateral *big.Int

	LockedStorage *big.Int
	Power         *big.Int
}

var _ ExecutableActor = (*MinerActor)(nil)

func NewMinerActor(owner types.Address, pledge *big.Int, coll *big.Int) (*types.Actor, error) {
	st := &MinerStorage{
		Owner:       owner,
		PledgeBytes: pledge,
		Collateral:  coll,
	}

	storageBytes, err := MarshalStorage(st)
	if err != nil {
		return nil, err
	}

	return types.NewActorWithMemory(types.MinerActorCodeCid, nil, storageBytes), nil
}

func (ma *MinerActor) Exports() Exports {
	panic("TODO")
}
