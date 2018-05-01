package storagemarket

import (
	"bytes"
	"fmt"
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// MinimumPledge is the minimum amount of space a user can pledge
var MinimumPledge = types.NewBytesAmount(10000)

// ErrPledgeTooLow is returned when the pledge is too low
var ErrPledgeTooLow = errors.NewRevertErrorf("pledge must be at least %s bytes", MinimumPledge)

func init() {
	cbor.RegisterCborType(Storage{})
	cbor.RegisterCborType(struct{}{})
	cbor.RegisterCborType(Filemap{})
}

// Actor implements the filecoin storage market. It is responsible
// for starting up new miners, adding bids, asks and deals. It also exposes the
// power table used to drive filecoin consensus.
type Actor struct{}

// Storage is the storage markets storage
type Storage struct {
	Miners types.AddrSet

	Orderbook *Orderbook

	Filemap *Filemap

	TotalCommittedStorage *types.BytesAmount
}

// NewStorage returns an empty StorageMarketStorage struct
func (sma *Actor) NewStorage() interface{} {
	return &Storage{}
}

var _ exec.ExecutableActor = (*Actor)(nil)

// NewActor returns a new storage market actor
func NewActor() (*types.Actor, error) {
	initStorage := &Storage{
		Miners: make(types.AddrSet),
		Orderbook: &Orderbook{
			Asks: make(AskSet),
			Bids: make(BidSet),
		},
		Filemap: &Filemap{
			Files: make(map[string][]uint64),
		},
	}
	storageBytes, err := actor.MarshalStorage(initStorage)
	if err != nil {
		return nil, err
	}
	return types.NewActorWithMemory(types.StorageMarketActorCodeCid, nil, storageBytes), nil
}

// Exports returns the actors exports
func (sma *Actor) Exports() exec.Exports {
	return storageMarketExports
}

var storageMarketExports = exec.Exports{
	"createMiner": &exec.FunctionSignature{
		Params: []abi.Type{abi.BytesAmount},
		Return: []abi.Type{abi.Address},
	},
	"addAsk": &exec.FunctionSignature{
		Params: []abi.Type{abi.TokenAmount, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addBid": &exec.FunctionSignature{
		Params: []abi.Type{abi.TokenAmount, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addDeal": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Integer, abi.Bytes, abi.Bytes},
		Return: []abi.Type{abi.Integer},
	},
	"commitDeals": &exec.FunctionSignature{
		Params: []abi.Type{abi.UintArray},
		Return: []abi.Type{abi.Integer},
	},
}

// CreateMiner creates a new miner with the a pledge of the given size. The
// miners collateral is set by the value in the message.
func (sma *Actor) CreateMiner(ctx exec.VMContext, pledge *types.BytesAmount) (types.Address, uint8, error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		if pledge.LessThan(MinimumPledge) {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, ErrPledgeTooLow
		}

		// 'CreateNewActor' (should likely be a method on the vmcontext)
		addr, err := ctx.AddressForNewActor()
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not get address for new actor")
		}

		minerActor, err := miner.NewActor(ctx.Message().From, pledge, ctx.Message().Value)
		if err != nil {
			// TODO? From an Actor's perspective this (and other stuff) should probably
			// never fail. It should call into the vmcontext to do this and the vm context
			// should "throw" to a higher level handler if there's a system fault. It would
			// simplify the actor code.
			return nil, errors.FaultErrorWrap(err, "could not get a new miner actor")
		}

		if err := ctx.TEMPCreateActor(addr, minerActor); err != nil {
			return nil, errors.FaultErrorWrap(err, "could not set miner actor in CreateMiner")
		}
		// -- end --

		_, _, err = ctx.Send(addr, "", ctx.Message().Value, nil)
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
func (sma *Actor) AddAsk(ctx exec.VMContext, price *types.TokenAmount, size *types.BytesAmount) (*big.Int, uint8,
	error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		// method must be called by a miner that was created by this storage market actor
		miner := ctx.Message().From

		_, ok := storage.Miners[miner]
		if !ok {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, errors.NewRevertErrorf("unknown miner: %s", miner)
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
		return nil, 1, errors.NewRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return askID, 0, nil
}

// AddBid adds a bid order to the orderbook. Can be called by anyone. The
// message must contain the appropriate amount of funds to be locked up for the
// bid.
func (sma *Actor) AddBid(ctx exec.VMContext, price *types.TokenAmount, size *types.BytesAmount) (*big.Int, uint8, error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		lockedFunds := price.CalculatePrice(size)
		if ctx.Message().Value.LessThan(lockedFunds) {
			fmt.Println(lockedFunds, ctx.Message().Value)
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, errors.NewRevertErrorf("must send price * size funds to create bid")
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
		return nil, 1, errors.NewRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return bidID, 0, nil
}

// AddDeal creates a deal from the given ask and bid
// It must always called by the owner of the miner in the ask
func (sma *Actor) AddDeal(ctx exec.VMContext, askID, bidID *big.Int, bidOwnerSig []byte, refb []byte) (*big.Int, uint8, error) {
	ref, err := cid.Cast(refb)
	if err != nil {
		return nil, 1, errors.NewRevertErrorf("'ref' input was not a valid cid: %s", err)
	}

	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		// TODO: askset is a map from uint64, our input is a big int.
		ask, ok := storage.Orderbook.Asks[askID.Uint64()]
		if !ok {
			return nil, errors.NewRevertErrorf("unknown ask %s", askID)
		}

		bid, ok := storage.Orderbook.Bids[bidID.Uint64()]
		if !ok {
			return nil, errors.NewRevertErrorf("unknown bid %s", bidID)
		}

		mown, ret, err := ctx.Send(ask.Owner, "getOwner", nil, nil)
		if err != nil {
			return nil, err
		}
		if ret != 0 {
			return nil, errors.NewRevertErrorf("ask.miner.getOwner() failed")
		}

		if !bytes.Equal(ctx.Message().From.Bytes(), mown) {
			return nil, fmt.Errorf("cannot create a deal for someone elses ask")
		}

		if ask.Size.LessThan(bid.Size) {
			return nil, errors.NewRevertErrorf("not enough space in ask for bid")
		}

		// TODO: real signature check and stuff
		if !bytes.Equal(bid.Owner.Bytes(), bidOwnerSig) {
			return nil, errors.NewRevertErrorf("signature failed to validate")
		}

		// mark bid as used (note: bid is a pointer)
		bid.Used = true

		// subtract used space from add
		ask.Size = ask.Size.Sub(bid.Size)

		d := &Deal{
			// Expiry:  ???
			DataRef: ref,
			Ask:     askID.Uint64(),
			Bid:     bidID.Uint64(),
		}

		dealID := uint64(len(storage.Filemap.Deals))
		ks := ref.KeyString()
		deals := storage.Filemap.Files[ks]
		storage.Filemap.Files[ks] = append(deals, dealID)
		storage.Filemap.Deals = append(storage.Filemap.Deals, d)

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

// CommitDeals marks the given deals as committed, counts up the total space
// occupied by those deals, updates the total storage count, and returns the
// total size of these deals.
func (sma *Actor) CommitDeals(ctx exec.VMContext, deals []uint64) (*types.BytesAmount, uint8, error) {
	var storage Storage
	ret, err := actor.WithStorage(ctx, &storage, func() (interface{}, error) {
		totalSize := types.NewBytesAmount(0)
		for _, d := range deals {
			if d >= uint64(len(storage.Filemap.Deals)) {
				return nil, errors.NewRevertErrorf("invalid deal id %d", d)
			}

			deal := storage.Filemap.Deals[d]
			ask := storage.Orderbook.Asks[deal.Ask]

			// make sure that the miner actor who owns the asks calls this
			if ask.Owner != ctx.Message().From {
				return nil, errors.NewRevertError("miner tried to commit with someone elses deal")
			}

			if deal.Committed {
				return nil, errors.NewRevertErrorf("deal %d already committed", d)
			}

			deal.Committed = true

			// TODO: Check that deal has not expired

			bid := storage.Orderbook.Bids[deal.Bid]

			// NB: if we allow for deals to be made at sizes other than the bid
			// size, this will need to be changed.
			totalSize = totalSize.Add(bid.Size)
		}

		// update the total data stored by the network
		storage.TotalCommittedStorage = storage.TotalCommittedStorage.Add(totalSize)

		return totalSize, nil
	})
	if err != nil {
		return nil, 1, err
	}

	count, ok := ret.(*types.BytesAmount)
	if !ok {
		return nil, 1, fmt.Errorf("expected *BytesAmount to be returned, but got %T instead", ret)
	}

	return count, 0, nil

}
