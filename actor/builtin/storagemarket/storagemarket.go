package storagemarket

import (
	"math/big"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// MinimumPledge is the minimum amount of space a user can pledge.
var MinimumPledge = types.NewBytesAmount(10000)

const (
	// ErrPledgeTooLow is the error code for a pledge under the MinimumPledge.
	ErrPledgeTooLow = 33
	// ErrUnknownMiner indicates a pledge under the MinimumPledge.
	ErrUnknownMiner = 34
	// ErrUnknownAsk indicates the ask for a deal could not be found.
	ErrUnknownAsk = 35
	// ErrUnknownBid indicates the bid for a deal could not be found.
	ErrUnknownBid = 36
	// ErrAskOwnerNotFound indicates the owner of an ask could not be found.
	ErrAskOwnerNotFound = 37
	// ErrNotBidOwner indicates the sender is not the owner of the bid.
	ErrNotBidOwner = 38
	// ErrInsufficientSpace indicates the bid to too big for the ask.
	ErrInsufficientSpace = 39
	// ErrInvalidSignature indicates the signature is invalid.
	ErrInvalidSignature = 40
	// ErrUnknownDeal indicates the deal id is not found.
	ErrUnknownDeal = 41
	// ErrNotDealOwner indicates someone other than the deal owner tried to commit.
	ErrNotDealOwner = 42
	// ErrDealCommitted indicates the deal is already committed.
	ErrDealCommitted = 43
	// ErrInsufficientBidFunds indicates the value of the bid message is less than the price of the space.
	ErrInsufficientBidFunds = 44
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrPledgeTooLow:         errors.NewCodedRevertErrorf(ErrPledgeTooLow, "pledge must be at least %s bytes", MinimumPledge),
	ErrUnknownMiner:         errors.NewCodedRevertErrorf(ErrUnknownMiner, "unknown miner"),
	ErrUnknownAsk:           errors.NewCodedRevertErrorf(ErrUnknownAsk, "ask id not found"),
	ErrUnknownBid:           errors.NewCodedRevertErrorf(ErrUnknownBid, "bid id not found"),
	ErrAskOwnerNotFound:     errors.NewCodedRevertErrorf(ErrAskOwnerNotFound, "cannot create a deal for someone elses ask"),
	ErrNotBidOwner:          errors.NewCodedRevertErrorf(ErrNotBidOwner, "ask id not found"),
	ErrInsufficientSpace:    errors.NewCodedRevertErrorf(ErrNotBidOwner, "not enough space in ask for bid"),
	ErrInvalidSignature:     errors.NewCodedRevertErrorf(ErrInvalidSignature, "signature failed to validate"),
	ErrUnknownDeal:          errors.NewCodedRevertErrorf(ErrUnknownDeal, "unknown deal id"),
	ErrNotDealOwner:         errors.NewCodedRevertErrorf(ErrNotDealOwner, "miner tried to commit with someone elses deal"),
	ErrDealCommitted:        errors.NewCodedRevertErrorf(ErrDealCommitted, "deal already committed"),
	ErrInsufficientBidFunds: errors.NewCodedRevertErrorf(ErrInsufficientBidFunds, "must send price * size funds to create bid"),
}

func init() {
	cbor.RegisterCborType(State{})
	cbor.RegisterCborType(struct{}{})
}

// Actor implements the filecoin storage market. It is responsible
// for starting up new miners, adding bids, asks and deals. It also exposes the
// power table used to drive filecoin consensus.
type Actor struct{}

// State is the storage markets storage.
type State struct {
	Miners types.AddrSet

	Orderbook *Orderbook

	TotalCommittedStorage *types.BytesAmount
}

// NewActor returns a new storage market actor.
func NewActor() (*types.Actor, error) {
	return types.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL()), nil
}

// InitializeState stores the actor's initial data structure.
func (sma *Actor) InitializeState(storage exec.Storage, _ interface{}) error {
	initStorage := &State{
		Miners: make(types.AddrSet),
		Orderbook: &Orderbook{
			StorageAsks: make(AskSet),
			Bids:        make(BidSet),
		},
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
		Params: []abi.Type{abi.BytesAmount, abi.Bytes, abi.PeerID},
		Return: []abi.Type{abi.Address},
	},
	"addAsk": &exec.FunctionSignature{
		Params: []abi.Type{abi.AttoFIL, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addBid": &exec.FunctionSignature{
		Params: []abi.Type{abi.AttoFIL, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addDeal": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Integer, abi.Bytes, abi.Bytes},
		Return: []abi.Type{abi.Integer},
	},
	"updatePower": &exec.FunctionSignature{
		Params: []abi.Type{abi.Bytes},
		Return: nil,
	},
}

// CreateMiner creates a new miner with the a pledge of the given size. The
// miners collateral is set by the value in the message.
func (sma *Actor) CreateMiner(ctx exec.VMContext, pledge *types.BytesAmount, publicKey []byte, pid peer.ID) (types.Address, uint8, error) {
	var state State
	ret, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		if pledge.LessThan(MinimumPledge) {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, Errors[ErrPledgeTooLow]
		}

		addr, err := ctx.AddressForNewActor()
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not get address for new actor")
		}

		minerInitializationParams := miner.NewState(ctx.Message().From, publicKey, pledge, pid, ctx.Message().Value)
		if err := ctx.CreateNewActor(addr, types.MinerActorCodeCid, minerInitializationParams); err != nil {
			return nil, err
		}

		_, _, err = ctx.Send(addr, "", ctx.Message().Value, nil)
		if err != nil {
			return nil, err
		}

		state.Miners[addr] = struct{}{}
		return addr, nil
	})
	if err != nil {
		return types.Address{}, errors.CodeError(err), err
	}

	return ret.(types.Address), 0, nil
}

// AddAsk adds an ask order to the orderbook. Must be called by a miner created
// by this storage market actor.
func (sma *Actor) AddAsk(ctx exec.VMContext, price *types.AttoFIL, size *types.BytesAmount) (*big.Int, uint8,
	error) {
	var storage State
	ret, err := actor.WithState(ctx, &storage, func() (interface{}, error) {
		// method must be called by a miner that was created by this storage market actor.
		miner := ctx.Message().From

		_, ok := storage.Miners[miner]
		if !ok {
			return nil, Errors[ErrUnknownMiner]
		}

		askID := storage.Orderbook.NextSAskID
		storage.Orderbook.NextSAskID++

		storage.Orderbook.StorageAsks[askID] = &Ask{
			ID:    askID,
			Price: price,
			Size:  size,
			Owner: miner,
		}

		return big.NewInt(0).SetUint64(askID), nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	askID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return askID, 0, nil
}

// AddBid adds a bid order to the orderbook. Can be called by anyone. The
// message must contain the appropriate amount of funds to be locked up for the
// bid.
func (sma *Actor) AddBid(ctx exec.VMContext, price *types.AttoFIL, size *types.BytesAmount) (*big.Int, uint8, error) {
	var state State
	ret, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		lockedFunds := price.CalculatePrice(size)
		if ctx.Message().Value.LessThan(lockedFunds) {
			return nil, Errors[ErrInsufficientBidFunds]
		}

		bidID := state.Orderbook.NextBidID
		state.Orderbook.NextBidID++

		state.Orderbook.Bids[bidID] = &Bid{
			ID:    bidID,
			Price: price,
			Size:  size,
			Owner: ctx.Message().From,
		}

		return big.NewInt(0).SetUint64(bidID), nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	bidID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return bidID, 0, nil
}

// UpdatePower is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing or willful removal)
func (sma *Actor) UpdatePower(ctx exec.VMContext, delta *types.BytesAmount) (uint8, error) {
	var state State
	_, err := actor.WithState(ctx, &state, func() (interface{}, error) {
		miner := ctx.Message().From

		_, ok := state.Miners[miner]
		if !ok {
			return nil, Errors[ErrUnknownMiner]
		}

		state.TotalCommittedStorage = state.TotalCommittedStorage.Add(delta)
		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}
