package core

import (
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(MinerStorage{})
}

// MinerActor is the miner actor
type MinerActor struct{}

// MinerStorage is the miner actors storage
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

// NewMinerActor returns a new miner actor
func NewMinerActor(owner types.Address, pledge *big.Int, coll *big.Int) (*types.Actor, error) {
	st := &MinerStorage{
		Owner:         owner,
		PledgeBytes:   pledge,
		Collateral:    coll,
		LockedStorage: big.NewInt(0),
	}

	storageBytes, err := MarshalStorage(st)
	if err != nil {
		return nil, err
	}

	return types.NewActorWithMemory(types.MinerActorCodeCid, nil, storageBytes), nil
}

var minerExports = Exports{
	"addAsk": &FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Integer},
		Return: []abi.Type{abi.Integer},
	},
	"getOwner": &FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Address},
	},
}

// Exports returns the miner actors exported functions
func (ma *MinerActor) Exports() Exports {
	return minerExports
}

// ErrCallerUnauthorized signals an unauthorized caller.
var ErrCallerUnauthorized = newRevertError("not authorized to call the method")

// ErrInsufficientPledge signals insufficient pledge for what you are trying to do.
var ErrInsufficientPledge = newRevertError("not enough pledged")

// AddAsk adds an ask via this miner to the storage markets orderbook
func (ma *MinerActor) AddAsk(ctx *VMContext, price, size *big.Int) (*big.Int, uint8, error) {
	var mstore MinerStorage
	out, err := WithStorage(ctx, &mstore, func() (interface{}, error) {
		if ctx.Message().From != mstore.Owner {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, ErrCallerUnauthorized
		}

		// compute locked storage + new ask
		locked := big.NewInt(0).Set(mstore.LockedStorage)
		total := locked.Add(locked, size)

		if total.Cmp(mstore.PledgeBytes) > 0 {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, ErrInsufficientPledge
		}

		mstore.LockedStorage = total

		// TODO: kinda feels weird that I can't get a real type back here
		out, ret, err := ctx.Send(StorageMarketAddress, "addAsk", nil, []interface{}{price, size})
		if err != nil {
			return nil, err
		}

		askID, err := abi.Deserialize(out, abi.Integer)
		if err != nil {
			return nil, faultErrorWrap(err, "error deserializing")
		}

		if ret != 0 {
			// TODO: Log an error maybe? need good ways of signaling *why* failures happened.
			// I guess we do want to revert all state changes in this case.
			// Which is usually signalled through an error. Something smells.
			return nil, newRevertError("call to StorageMarket.addAsk failed")
		}

		return askID.Val, nil
	})
	if err != nil {
		return nil, 1, err
	}

	askID, ok := out.(*big.Int)
	if !ok {
		return nil, 1, newRevertErrorf("expected an Integer return value from call, but got %T instead", out)
	}

	return askID, 0, nil
}

func (ma *MinerActor) GetOwner(ctx *VMContext) (types.Address, uint8, error) {
	var mstore MinerStorage
	out, err := WithStorage(ctx, &mstore, func() (interface{}, error) {
		return mstore.Owner, nil
	})
	if err != nil {
		return "", 1, err
	}

	a, ok := out.(types.Address)
	if !ok {
		return "", 1, fmt.Errorf("expected an Address return value from call, but got %T instead", out)
	}

	return a, 0, nil
}
