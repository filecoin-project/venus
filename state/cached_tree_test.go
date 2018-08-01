package state

import (
	"context"
	"testing"

	"gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedStateGetCommit(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	cst := hamt.NewCborStore()
	ctx := context.Background()

	// set up state tree and cache wrapper
	underlying := NewEmptyStateTree(cst)
	tree := NewCachedStateTree(underlying)

	// create some actors
	act1 := types.NewActor(types.AccountActorCodeCid, nil)
	act1Storage := []byte("hello")
	act1.WriteStorage(act1Storage)
	act1.IncNonce()
	act2 := types.NewActor(types.AccountActorCodeCid, nil)
	act2.WriteStorage([]byte("world"))

	addrGetter := types.NewAddressForTestGetter()
	addr1, addr2 := addrGetter(), addrGetter()

	// add actors to underlying cache
	assert.NoError(underlying.SetActor(ctx, addr1, act1))
	assert.NoError(underlying.SetActor(ctx, addr2, act2))

	// get act1 from cache
	cAct1, err := tree.GetActor(ctx, addr1)
	require.NoError(err)

	assert.Equal(uint64(1), uint64(cAct1.Nonce))
	assert.Equal(act1Storage, cAct1.ReadStorage())

	// altering act1 doesn't alter it in underlying cache
	cAct1.IncNonce()
	newStorage := []byte("goodbye")
	cAct1.WriteStorage(newStorage)

	uAct1, err := underlying.GetActor(ctx, addr1)
	require.NoError(err)

	assert.Equal(uint64(1), uint64(uAct1.Nonce))
	assert.Equal(act1Storage, uAct1.ReadStorage())

	// retrieving from the cache again returns the same instance
	cAct1Again, err := tree.GetActor(ctx, addr1)
	require.NoError(err)
	assert.True(cAct1 == cAct1Again, "Cache returns same instance on second get")

	// commit changes sets changes in underlying tree
	err = tree.Commit(ctx)
	require.NoError(err)

	uAct1Again, err := underlying.GetActor(ctx, addr1)
	require.NoError(err)

	assert.Equal(uint64(2), uint64(uAct1Again.Nonce))
	assert.Equal([]byte("goodbye"), uAct1Again.ReadStorage())

	// commit doesn't affect untouched actors
	uAct2, err := underlying.GetActor(ctx, addr2)
	require.NoError(err)

	assert.Equal(uint64(0), uint64(uAct2.Nonce))
	assert.Equal([]byte("world"), uAct2.ReadStorage())
}

func TestCachedStateGetOrCreate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	cst := hamt.NewCborStore()
	ctx := context.Background()

	// set up state tree and cache wrapper
	underlying := NewEmptyStateTree(cst)
	tree := NewCachedStateTree(underlying)

	actorToCreate := types.NewActor(types.AccountActorCodeCid, nil)

	// can create actor in cache
	addr := types.NewAddressForTestGetter()()
	actor, err := tree.GetOrCreateActor(ctx, addr, func() (*types.Actor, error) {
		return actorToCreate, nil
	})
	require.NoError(err)
	assert.True(actor == actorToCreate, "GetOrCreate returns same instance created in creator")

	// cache returns same instance
	cAct, err := tree.GetActor(ctx, addr)
	require.NoError(err)
	assert.True(actor == cAct, "actor retrieved from cache is same as actor created in cache")

	// GetOrCreate does not add actor to underlying tree
	_, err = underlying.GetActor(ctx, addr)
	require.Equal("actor not found", err.Error())
}
