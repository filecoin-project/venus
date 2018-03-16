package types

import (
	"context"
	"fmt"
	"testing"

	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/stretchr/testify/assert"
)

func TestStatePutGet(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	act1 := NewActor(AccountActorCodeCid, nil)
	act1.WriteStorage([]byte("hello"))
	act1.IncNonce()
	act2 := NewActor(AccountActorCodeCid, nil)
	act2.WriteStorage([]byte("world"))
	act2.IncNonce()
	act2.IncNonce()

	addrGetter := NewAddressForTestGetter()
	addr1 := addrGetter()
	addr2 := addrGetter()

	assert.NoError(tree.SetActor(ctx, addr1, act1))
	assert.NoError(tree.SetActor(ctx, addr2, act2))

	act1out, err := tree.GetActor(ctx, addr1)
	assert.NoError(err)
	assert.Equal(act1, act1out)
	act2out, err := tree.GetActor(ctx, addr2)
	assert.NoError(err)
	assert.Equal(act2, act2out)

	// now test it persists across recreation of tree
	tcid, err := tree.Flush(ctx)
	assert.NoError(err)

	tree2, err := LoadStateTree(ctx, cst, tcid)
	assert.NoError(err)

	act1out2, err := tree2.GetActor(ctx, addr1)
	assert.NoError(err)
	assert.Equal(act1, act1out2)
	act2out2, err := tree2.GetActor(ctx, addr2)
	assert.NoError(err)
	assert.Equal(act2, act2out2)
}

func TestStateErrors(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	_, err := tree.GetActor(ctx, NewAddressForTestGetter()())
	assert.EqualError(err, "not found")

	c, err := cid.NewPrefixV0(mh.BLAKE2B_MIN + 31).Sum([]byte("cats"))
	assert.NoError(err)

	tr2, err := LoadStateTree(ctx, cst, c)
	assert.EqualError(err, "failed to load node: not found")
	assert.Nil(tr2)
}

func TestStateGetOrCreate(t *testing.T) {
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	addr := NewAddressForTestGetter()()

	// no actor - error
	t.Run("no actor - error", func(t *testing.T) {
		assert := assert.New(t)

		actor, err := tree.GetOrCreateActor(ctx, addr, func() (*Actor, error) {
			return nil, fmt.Errorf("fail")
		})
		assert.EqualError(err, "fail")
		assert.Nil(actor)
	})

	t.Run("no actor - success", func(t *testing.T) {
		assert := assert.New(t)

		actor, err := tree.GetOrCreateActor(ctx, addr, func() (*Actor, error) {
			return &Actor{}, nil
		})
		assert.NoError(err)
		assert.Equal(actor, &Actor{})
	})

	t.Run("actor exists", func(t *testing.T) {
		assert := assert.New(t)

		actor := NewActor(nil, NewTokenAmount(10))
		assert.NoError(tree.SetActor(ctx, addr, actor))

		actorBack, err := tree.GetOrCreateActor(ctx, addr, func() (*Actor, error) {
			return &Actor{}, nil
		})
		assert.NoError(err)
		assert.Equal(actorBack, actor)
	})
}

func TestSnapshotAndRevertTo(t *testing.T) {
	assert := assert.New(t)
	newAddress := NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// Do nothing and revert.
	st := NewEmptyStateTree(cst)
	emptyCid, err := st.Flush(ctx)
	assert.NoError(err)
	emptyRev := st.Snapshot()
	st.RevertTo(emptyRev)
	gotCid, err := st.Flush(ctx)
	assert.NoError(err)
	assert.True(emptyCid.Equals(gotCid))
	// Add an actor that should not affect anything.
	st.SetActor(ctx, newAddress(), NewActor(AccountActorCodeCid, nil))
	st.RevertTo(emptyRev)

	// Add two actors, snapshotting each step.
	st.SetActor(ctx, newAddress(), NewActor(AccountActorCodeCid, nil))
	oneActorCid, err := st.Flush(ctx)
	assert.NoError(err)
	assert.False(oneActorCid.Equals(emptyCid))
	oneActorRev := st.Snapshot()
	st.SetActor(ctx, newAddress(), NewActor(AccountActorCodeCid, nil))
	twoActorCid, err := st.Flush(ctx)
	assert.NoError(err)
	assert.False(twoActorCid.Equals(oneActorCid))
	twoActorRev := st.Snapshot()

	// Roll back to same state.
	st.RevertTo(twoActorRev)
	gotCid, err = st.Flush(ctx)
	assert.NoError(err)
	assert.True(twoActorCid.Equals(gotCid))

	// Roll back to one actor state.
	st.RevertTo(oneActorRev)
	gotCid, err = st.Flush(ctx)
	assert.NoError(err)
	assert.True(oneActorCid.Equals(gotCid))

	// Roll back to empty.
	st.RevertTo(emptyRev)
	gotCid, err = st.Flush(ctx)
	assert.NoError(err)
	assert.True(emptyCid.Equals(gotCid))
}

func TestLoadedStateTreeCanSnapshot(t *testing.T) {
	// There was an issue where LoadStateTree initialized the tree
	// differently than the constructor, hence this test. If it
	// doesn't crash in Snapshot() we are winning.
	assert := assert.New(t)
	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	act := NewActor(AccountActorCodeCid, nil)
	assert.NoError(tree.SetActor(ctx, NewAddressForTestGetter()(), act))
	cid, err := tree.Flush(ctx)
	assert.NoError(err)

	tree2, err := LoadStateTree(ctx, cst, cid)
	assert.NoError(err)
	snap := tree2.Snapshot()
	tree2.RevertTo(snap)
	gotCid, err := tree.Flush(ctx)
	assert.NoError(err)
	assert.True(cid.Equals(gotCid))
}
