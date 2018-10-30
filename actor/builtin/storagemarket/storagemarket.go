package storagemarket

import (
	"context"
	"fmt"
	"math/big"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// MinimumPledge is the minimum amount of sectors a user can pledge.
var MinimumPledge = big.NewInt(10)

// MinimumCollateralPerSector is the minimum amount of collateral required per sector
var MinimumCollateralPerSector, _ = types.NewAttoFILFromFILString("0.001")

const (
	// ErrPledgeTooLow is the error code for a pledge under the MinimumPledge.
	ErrPledgeTooLow = 33
	// ErrUnknownMiner indicates a pledge under the MinimumPledge.
	ErrUnknownMiner = 34
	// ErrAskOwnerNotFound indicates the owner of an ask could not be found.
	ErrAskOwnerNotFound = 35
	// ErrInsufficientSpace indicates the bid to too big for the ask.
	ErrInsufficientSpace = 36
	// ErrInvalidSignature indicates the signature is invalid.
	ErrInvalidSignature = 37
	// ErrUnknownDeal indicates the deal id is not found.
	ErrUnknownDeal = 38
	// ErrNotDealOwner indicates someone other than the deal owner tried to commit.
	ErrNotDealOwner = 39
	// ErrDealCommitted indicates the deal is already committed.
	ErrDealCommitted = 40
	// ErrInsufficientBidFunds indicates the value of the bid message is less than the price of the space.
	ErrInsufficientBidFunds = 41
	// ErrInsufficientCollateral indicates the collateral is too low.
	ErrInsufficientCollateral = 42
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrPledgeTooLow:           errors.NewCodedRevertErrorf(ErrPledgeTooLow, "pledge must be at least %s sectors", MinimumPledge),
	ErrUnknownMiner:           errors.NewCodedRevertErrorf(ErrUnknownMiner, "unknown miner"),
	ErrInvalidSignature:       errors.NewCodedRevertErrorf(ErrInvalidSignature, "signature failed to validate"),
	ErrUnknownDeal:            errors.NewCodedRevertErrorf(ErrUnknownDeal, "unknown deal id"),
	ErrNotDealOwner:           errors.NewCodedRevertErrorf(ErrNotDealOwner, "miner tried to commit with someone elses deal"),
	ErrDealCommitted:          errors.NewCodedRevertErrorf(ErrDealCommitted, "deal already committed"),
	ErrInsufficientBidFunds:   errors.NewCodedRevertErrorf(ErrInsufficientBidFunds, "must send price * size funds to create bid"),
	ErrInsufficientCollateral: errors.NewCodedRevertErrorf(ErrInsufficientCollateral, "collateral must be more than %s FIL per sector", MinimumCollateralPerSector),
}

func init() {
	cbor.RegisterCborType(State{})
	cbor.RegisterCborType(struct{}{})
}

// Actor implements the filecoin storage market. It is responsible
// for starting up new miners, adding bids, asks and deals. It also exposes the
// power table used to drive filecoin consensus.
type Actor struct{}

// State is the storage market's storage.
type State struct {
	Miners *cid.Cid

	// TotalCommitedStorage is the number of sectors that are currently committed
	// in the whole network.
	TotalCommittedStorage *big.Int
}

// NewActor returns a new storage market actor.
func NewActor() (*actor.Actor, error) {
	return actor.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL()), nil
}

// InitializeState stores the actor's initial data structure.
func (sma *Actor) InitializeState(storage exec.Storage, _ interface{}) error {
	initStorage := &State{
		TotalCommittedStorage: big.NewInt(0),
	}
	stateBytes, err := cbor.DumpObject(initStorage)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, nil)
}

var _ exec.ExecutableActor = (*Actor)(nil)

// Exports returns the actors exports.
func (sma *Actor) Exports() exec.Exports {
	return storageMarketExports
}

var storageMarketExports = exec.Exports{
	"createMiner": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Bytes, abi.PeerID},
		Return: []abi.Type{abi.Address},
	},
	"updatePower": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: nil,
	},
	"getTotalStorage": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.Integer},
	},
}

// CreateMiner creates a new miner with the a pledge of the given amount of sectors. The
// miners collateral is set by the value in the message.
func (sma *Actor) CreateMiner(vmctx exec.VMContext, pledge *big.Int, publicKey []byte, pid peer.ID) (address.Address, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		if pledge.Cmp(MinimumPledge) < 0 {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, Errors[ErrPledgeTooLow]
		}

		addr, err := vmctx.AddressForNewActor()
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not get address for new actor")
		}

		if vmctx.Message().Value.LessThan(MinimumCollateral(pledge)) {
			return nil, Errors[ErrInsufficientCollateral]
		}

		minerInitializationParams := miner.NewState(vmctx.Message().From, publicKey, pledge, pid, vmctx.Message().Value)
		if err := vmctx.CreateNewActor(addr, types.MinerActorCodeCid, minerInitializationParams); err != nil {
			return nil, err
		}

		_, _, err = vmctx.Send(addr, "", vmctx.Message().Value, nil)
		if err != nil {
			return nil, err
		}

		ctx := context.Background()

		state.Miners, err = actor.SetKeyValue(ctx, vmctx.Storage(), state.Miners, addr.String(), true)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not set miner key value for lookup with CID: %s", state.Miners)
		}

		return addr, nil
	})
	if err != nil {
		return address.Address{}, errors.CodeError(err), err
	}

	return ret.(address.Address), 0, nil
}

// UpdatePower is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing or willful removal). The delta is in number of sectors.
func (sma *Actor) UpdatePower(vmctx exec.VMContext, delta *big.Int) (uint8, error) {
	var state State
	_, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		miner := vmctx.Message().From
		ctx := context.Background()

		miners, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Miners)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with CID: %s", state.Miners)
		}

		_, err = miners.Find(ctx, miner.String())
		if err != nil {
			if err == hamt.ErrNotFound {
				return nil, Errors[ErrUnknownMiner]
			}
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with address: %s", miner)
		}

		state.TotalCommittedStorage = state.TotalCommittedStorage.Add(state.TotalCommittedStorage, delta)

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// GetTotalStorage returns the total amount of proven storage in the system.
func (sma *Actor) GetTotalStorage(vmctx exec.VMContext) (*big.Int, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		return state.TotalCommittedStorage, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	count, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, fmt.Errorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return count, 0, nil
}

// MinimumCollateral returns the minimum required amount of collateral for a given pledge
func MinimumCollateral(sectors *big.Int) *types.AttoFIL {
	return MinimumCollateralPerSector.MulBigInt(sectors)
}
