package core

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
)

// MinimumPledge is the minimum amount of space a user can pledge
var MinimumPledge = types.NewBytesAmount(10000)

// ErrPledgeTooLow is returned when the pledge is too low
var ErrPledgeTooLow = newRevertErrorf("pledge must be at least %s bytes", MinimumPledge)

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
	Miners types.AddrSet

	Orderbook *Orderbook
}

var _ ExecutableActor = (*StorageMarketActor)(nil)

// NewStorageMarketActor returns a new storage market actor
func NewStorageMarketActor() (*types.Actor, error) {
	initStorage := &StorageMarketStorage{
		Miners: make(types.AddrSet),
		Orderbook: &Orderbook{
			Asks: make(AskSet),
			Bids: make(BidSet),
		},
	}
	storageBytes, err := MarshalStorage(initStorage)
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
		Params: []abi.Type{abi.BytesAmount},
		Return: []abi.Type{abi.Address},
	},
	"addAsk": &FunctionSignature{
		Params: []abi.Type{abi.TokenAmount, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addBid": &FunctionSignature{
		Params: []abi.Type{abi.TokenAmount, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addDeal": &FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Integer, abi.Bytes},
		Return: []abi.Type{abi.Integer},
	},
}

// CreateMiner creates a new miner with the a pledge of the given size. The
// miners collateral is set by the value in the message.
func (sma *StorageMarketActor) CreateMiner(ctx *VMContext, pledge *types.BytesAmount) (types.Address, uint8, error) {
	var storage StorageMarketStorage
	ret, err := WithStorage(ctx, &storage, func() (interface{}, error) {
		if pledge.LessThan(MinimumPledge) {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, ErrPledgeTooLow
		}

		// 'CreateNewActor' (should likely be a method on the vmcontext)
		addr, err := ctx.AddressForNewActor()
		if err != nil {
			return nil, faultErrorWrap(err, "could not get address for new actor")
		}

		miner, err := NewMinerActor(ctx.message.From, pledge, ctx.message.Value)
		if err != nil {
			// TODO? From an Actor's perspective this (and other stuff) should probably
			// never fail. It should call into the vmcontext to do this and the vm context
			// should "throw" to a higher level handler if there's a system fault. It would
			// simplify the actor code.
			return nil, faultErrorWrap(err, "could not get a new miner actor")
		}

		if err := ctx.state.SetActor(context.TODO(), addr, miner); err != nil {
			return nil, faultErrorWrap(err, "could not set miner actor in CreateMiner")
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
		return types.Address{}, 1, err
	}

	return ret.(types.Address), 0, nil
}

// AddAsk adds an ask order to the orderbook. Must be called by a miner created
// by this storage market actor
func (sma *StorageMarketActor) AddAsk(ctx *VMContext, price *types.TokenAmount, size *types.BytesAmount) (*big.Int, uint8,
	error) {
	var storage StorageMarketStorage
	ret, err := WithStorage(ctx, &storage, func() (interface{}, error) {
		// method must be called by a miner that was created by this storage market actor
		miner := ctx.Message().From

		_, ok := storage.Miners[miner]
		if !ok {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, newRevertErrorf("unknown miner: %s", miner)
		}

		askID := storage.Orderbook.NextAskID
		storage.Orderbook.NextAskID++

		storage.Orderbook.Asks[askID] = &Ask{
			ID:    askID,
			Price: price,
			Size:  size,
			Owner: miner,
		}

		return big.NewInt(0).SetUint64(askID), nil
	})
	if err != nil {
		return nil, 1, err
	}

	askID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, newRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return askID, 0, nil
}

// AddBid adds a bid order to the orderbook. Can be called by anyone. The
// message must contain the appropriate amount of funds to be locked up for the
// bid.
func (sma *StorageMarketActor) AddBid(ctx *VMContext, price *types.TokenAmount, size *types.BytesAmount) (*big.Int, uint8, error) {
	var storage StorageMarketStorage
	ret, err := WithStorage(ctx, &storage, func() (interface{}, error) {
		lockedFunds := price.CalculatePrice(size)
		if ctx.Message().Value.LessThan(lockedFunds) {
			fmt.Println(lockedFunds, ctx.Message().Value)
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, newRevertErrorf("must send price * size funds to create bid")
		}

		bidID := storage.Orderbook.NextBidID
		storage.Orderbook.NextBidID++

		storage.Orderbook.Bids[bidID] = &Bid{
			ID:    bidID,
			Price: price,
			Size:  size,
			Owner: ctx.Message().From,
		}

		return big.NewInt(0).SetUint64(bidID), nil
	})
	if err != nil {
		return nil, 1, err
	}

	bidID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, newRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return bidID, 0, nil
}

// AddDeal creates a deal from the given ask and bid
func (sma *StorageMarketActor) AddDeal(ctx *VMContext, askID, bidID *big.Int, clientSig []byte) (*big.Int, uint8, error) {
	var storage StorageMarketStorage
	ret, err := WithStorage(ctx, &storage, func() (interface{}, error) {
		// TODO: askset is a map from uint64, our input is a big int.
		ask, ok := storage.Orderbook.Asks[askID.Uint64()]
		if !ok {
			return nil, fmt.Errorf("unknown ask %s", askID)
		}

		bid, ok := storage.Orderbook.Bids[bidID.Uint64()]
		if !ok {
			return nil, fmt.Errorf("unknown bid %s", bidID)
		}

		mown, ret, err := ctx.Send(ask.Owner, "getOwner", nil, nil)
		if err != nil {
			return nil, err
		}
		if ret != 0 {
			return nil, fmt.Errorf("ask.miner.getOwner() failed")
		}

		if !bytes.Equal(ctx.Message().From.Bytes(), mown) {
			return nil, fmt.Errorf("cannot create a deal for someone elses ask")
		}

		if ask.Size.LessThan(bid.Size) {
			return nil, fmt.Errorf("not enough space in ask for bid")
		}

		// TODO: real signature check and stuff
		if !bytes.Equal(bid.Owner.Bytes(), clientSig) {
			return nil, fmt.Errorf("signature failed to validate")
		}

		// mark bid as used
		bid.Used = true

		// subtract used space from add
		ask.Size = ask.Size.Sub(bid.Size)

		d := &Deal{
			// Expiry:  ???
			// DataRef: ???
			Ask: askID.Uint64(),
			Bid: bidID.Uint64(),
		}

		dealID := len(storage.Orderbook.Deals)
		storage.Orderbook.Deals = append(storage.Orderbook.Deals, d)

		return big.NewInt(int64(dealID)), nil
	})
	if err != nil {
		return nil, 1, err
	}

	dealID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, fmt.Errorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return dealID, 0, nil
}
