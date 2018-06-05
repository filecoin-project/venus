package core

import (
	"context"

	hamt "gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/require"
)

// MkChild creates a new block with parent, blk, and supplied nonce.
func MkChild(blks []*types.Block, stateRoot *cid.Cid, nonce uint64) *types.Block {
	parents := types.NewSortedCidSet()
	for _, blk := range blks {
		parents.Add(blk.Cid())
	}
	return &types.Block{
		Parents:         parents,
		Height:          blks[0].Height + 1,
		Nonce:           nonce,
		StateRoot:       stateRoot,
		Messages:        []*types.Message{},
		MessageReceipts: []*types.MessageReceipt{},
	}
}

// AddChain creates and processes new, empty chain of length, beginning from blk.
func AddChain(ctx context.Context, processNewBlock NewBlockProcessor, loadStateTreeTS AggregateStateTreeComputer, blks []*types.Block, length int) (*types.Block, error) {
	st, err := loadStateTreeTS(ctx, NewTipSet(blks[0]))
	if err != nil {
		return nil, err
	}
	stateRoot, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}
	l := uint64(length)
	var blk *types.Block
	for i := uint64(0); i < l; i++ {
		blk = MkChild(blks, stateRoot, i)
		_, err := processNewBlock(ctx, blk)
		if err != nil {
			return nil, err
		}
		blks = []*types.Block{blk}
	}
	return blks[0], nil
}

// RequireMakeStateTree takes a map of addresses to actors and stores them on
// the state tree, requiring that all its steps succeed.
func RequireMakeStateTree(require *require.Assertions, cst *hamt.CborIpldStore, acts map[types.Address]*types.Actor) (*cid.Cid, state.Tree) {
	ctx := context.Background()
	t := state.NewEmptyStateTreeWithActors(cst, builtin.Actors)

	for addr, act := range acts {
		err := t.SetActor(ctx, addr, act)
		require.NoError(err)
	}

	c, err := t.Flush(ctx)
	require.NoError(err)

	return c, t
}

// RequireNewEmptyActor creates a new empty actor with the given starting
// value and requires that its steps succeed.
func RequireNewEmptyActor(require *require.Assertions, value *types.TokenAmount) *types.Actor {
	return &types.Actor{Balance: value}
}

// RequireNewAccountActor creates a new account actor with the given starting
// value and requires that its steps succeed.
func RequireNewAccountActor(require *require.Assertions, value *types.TokenAmount) *types.Actor {
	act, err := account.NewActor(value)
	require.NoError(err)
	return act
}

// RequireNewMinerActor creates a new miner actor with the given owner, pledge, and collateral,
// and requires that its steps succeed.
func RequireNewMinerActor(require *require.Assertions, owner types.Address, key []byte, pledge *types.BytesAmount, coll *types.TokenAmount) *types.Actor {
	act, err := miner.NewActor(owner, key, pledge, coll)
	require.NoError(err)
	return act
}

// RequireNewFakeActor instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActor(require *require.Assertions, codeCid *cid.Cid) *types.Actor {
	return RequireNewFakeActorWithTokens(require, codeCid, types.NewTokenAmount(100))
}

// RequireNewFakeActorWithTokens instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActorWithTokens(require *require.Assertions, codeCid *cid.Cid, amt *types.TokenAmount) *types.Actor {
	storageBytes, err := actor.MarshalStorage(&actor.FakeActorStorage{})
	require.NoError(err)
	return types.NewActorWithMemory(codeCid, amt, storageBytes)
}

// MustGetNonce returns the next nonce for an actor at the given address or panics.
func MustGetNonce(st state.Tree, a types.Address) uint64 {
	mp := NewMessagePool()
	nonce, err := NextNonce(context.Background(), st, mp, a)
	if err != nil {
		panic(err)
	}
	return nonce
}

// MustAdd adds the given messages to the messagepool or panics if it
// cannot.
func MustAdd(p *MessagePool, msgs ...*types.Message) {
	for _, m := range msgs {
		if _, err := p.Add(m); err != nil {
			panic(err)
		}
	}
}

// MustConvertParams abi encodes the given parameters into a byte array (or panics)
func MustConvertParams(params ...interface{}) []byte {
	vals, err := abi.ToValues(params)
	if err != nil {
		panic(err)
	}

	out, err := abi.EncodeValues(vals)
	if err != nil {
		panic(err)
	}
	return out
}

// NewChainWithMessages creates a chain of tipsets containing the given messages
// and stores them in the given store.  Note the msg arguments are slices of
// slices of messages -- each slice of slices goes into a successive tipset,
// and each slice within this slice goes into a block of that tipset
func NewChainWithMessages(store *hamt.CborIpldStore, root TipSet, msgSets ...[][]*types.Message) []TipSet {
	tipSets := []TipSet{}
	parents := root
	// only add root to the chain if it is not the zero-valued-tipset
	if len(parents) != 0 {
		for _, blk := range parents {
			MustPut(store, blk)
		}
		tipSets = append(tipSets, parents)
	}

	for _, tsMsgs := range msgSets {
		ts := TipSet{}
		// If a message set does not contain a slice of messages then
		// add a tipset with no messages and a single block to the chain
		if len(tsMsgs) == 0 {
			child := &types.Block{
				Height:  parents.Height() + 1,
				Parents: parents.ToSortedCidSet(),
			}
			MustPut(store, child)
			ts[child.Cid().String()] = child
		}
		for _, msgs := range tsMsgs {
			child := &types.Block{
				Messages: msgs,
				Parents:  parents.ToSortedCidSet(),
				Height:   parents.Height() + 1,
			}
			MustPut(store, child)
			ts[child.Cid().String()] = child
		}
		tipSets = append(tipSets, ts)
		parents = ts
	}

	return tipSets
}

// MustPut stores the thingy in the store or panics if it cannot.
func MustPut(store *hamt.CborIpldStore, thingy interface{}) *cid.Cid {
	cid, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return cid
}

// MustDecodeCid decodes a string to a Cid pointer, panicking on error
func MustDecodeCid(cidStr string) *cid.Cid {
	decode, err := cid.Decode(cidStr)
	if err != nil {
		panic(err)
	}

	return decode
}
