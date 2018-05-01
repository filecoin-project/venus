package core

import (
	"context"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/require"
)

// MkChild creates a new block with parent, blk, and supplied nonce.
func MkChild(blk *types.Block, nonce uint64) *types.Block {
	return &types.Block{
		Parent:          blk.Cid(),
		Height:          blk.Height + 1,
		Nonce:           nonce,
		StateRoot:       blk.StateRoot,
		Messages:        []*types.Message{},
		MessageReceipts: []*types.MessageReceipt{},
	}
}

// AddChain creates and processes new, empty chain of length, beginning from blk.
func AddChain(ctx context.Context, processNewBlock NewBlockProcessor, blk *types.Block, length int) (*types.Block, error) {
	l := uint64(length)
	for i := uint64(0); i < l; i++ {
		blk = MkChild(blk, i)
		_, err := processNewBlock(ctx, blk)
		if err != nil {
			return nil, err
		}
	}
	return blk, nil
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

// RequireNewAccountActor creates a new account actor with the given starting
// value and requires that its steps succeed.
func RequireNewAccountActor(require *require.Assertions, value *types.TokenAmount) *types.Actor {
	act, err := account.NewActor(value)
	require.NoError(err)
	return act
}

// RequireNewMinerActor creates a new miner actor with the given owner, pledge, and collateral,
// and requires that its steps succeed.
func RequireNewMinerActor(require *require.Assertions, owner types.Address, pledge *types.BytesAmount, coll *types.TokenAmount) *types.Actor {
	act, err := miner.NewActor(owner, pledge, coll)
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

// NewChainWithMessages creates a chain of blocks containing the given messages
// and stores them in the given store. Note the msg arguments are slices of
// messages -- each slice goes into a successive block.
func NewChainWithMessages(store *hamt.CborIpldStore, root *types.Block, msgSets ...[]*types.Message) []*types.Block {
	parent := root
	blocks := []*types.Block{}
	if parent != nil {
		MustPut(store, parent)
		blocks = append(blocks, parent)
	}

	for _, msgs := range msgSets {
		child := &types.Block{Messages: msgs}
		if parent != nil {
			child.Parent = parent.Cid()
			child.Height = parent.Height + 1
		}
		MustPut(store, child)
		blocks = append(blocks, child)
		parent = child
	}

	return blocks
}

// MustPut stores the thingy in the store or panics if it cannot.
func MustPut(store *hamt.CborIpldStore, thingy interface{}) *cid.Cid {
	cid, err := store.Put(context.Background(), thingy)
	if err != nil {
		panic(err)
	}
	return cid
}
