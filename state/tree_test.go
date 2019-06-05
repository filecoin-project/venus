package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestStatePutGet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	act1 := actor.NewActor(types.AccountActorCodeCid, nil)
	act1.IncNonce()
	act2 := actor.NewActor(types.AccountActorCodeCid, nil)
	act2.IncNonce()
	act2.IncNonce()

	addrGetter := address.NewForTestGetter()
	addr1 := addrGetter()
	addr2 := addrGetter()

	assert.NoError(t, tree.SetActor(ctx, addr1, act1))
	assert.NoError(t, tree.SetActor(ctx, addr2, act2))

	act1out, err := tree.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.Equal(t, act1, act1out)
	act2out, err := tree.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.Equal(t, act2, act2out)

	// now test it persists across recreation of tree
	tcid, err := tree.Flush(ctx)
	assert.NoError(t, err)

	tree2, err := LoadStateTree(ctx, cst, tcid, nil)
	assert.NoError(t, err)

	act1out2, err := tree2.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.Equal(t, act1, act1out2)
	act2out2, err := tree2.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.Equal(t, act2, act2out2)
}

func TestStateErrors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	a, err := tree.GetActor(ctx, address.NewForTestGetter()())
	assert.Nil(t, a)
	assert.Error(t, err)
	assert.True(t, IsActorNotFoundError(err))

	c, err := cid.V1Builder{Codec: cid.DagCBOR, MhType: mh.BLAKE2B_MIN + 31}.Sum([]byte("cats"))
	assert.NoError(t, err)

	tr2, err := LoadStateTree(ctx, cst, c, nil)
	assert.EqualError(t, err, "failed to load node: not found")
	assert.Nil(t, tr2)
}

func TestStateGetOrCreate(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)

	addr := address.NewForTestGetter()()

	// no actor - error
	t.Run("no actor - error", func(t *testing.T) {
		actor, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, error) {
			return nil, fmt.Errorf("fail")
		})
		assert.EqualError(t, err, "fail")
		assert.Nil(t, actor)
	})

	t.Run("no actor - success", func(t *testing.T) {
		a, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, error) {
			return &actor.Actor{}, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, a, &actor.Actor{})
	})

	t.Run("actor exists", func(t *testing.T) {
		a := actor.NewActor(cid.Undef, types.NewAttoFILFromFIL(10))
		assert.NoError(t, tree.SetActor(ctx, addr, a))

		actorBack, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, error) {
			return &actor.Actor{}, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, actorBack, a)
	})
}

func TestGetAllActors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	tree := NewEmptyStateTree(cst)
	addr := address.NewForTestGetter()()

	actor := actor.Actor{Code: types.AccountActorCodeCid, Nonce: 1234, Balance: types.NewAttoFILFromFIL(123)}
	err := tree.SetActor(ctx, addr, &actor)
	assert.NoError(t, err)
	_, err = tree.Flush(ctx)
	require.NoError(t, err)

	results := GetAllActors(ctx, tree)

	for result := range results {
		assert.Equal(t, addr.String(), result.Address)
		assert.Equal(t, actor.Code, result.Actor.Code)
		assert.Equal(t, actor.Nonce, result.Actor.Nonce)
		assert.Equal(t, actor.Balance, result.Actor.Balance)
	}
}
