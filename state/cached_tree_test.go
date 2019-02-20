package state

import (
	"context"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
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
	act1 := actor.NewActor(types.AccountActorCodeCid, nil)
	act1Cid := requireCid(t, "hello")
	act1.Head = act1Cid
	act1.IncNonce()
	act2 := actor.NewActor(types.AccountActorCodeCid, nil)
	act2Cid := requireCid(t, "world")
	act2.Head = act2Cid

	addrGetter := address.NewForTestGetter()
	addr1, addr2 := addrGetter(), addrGetter()

	// add actors to underlying cache
	assert.NoError(underlying.SetActor(ctx, addr1, act1))
	assert.NoError(underlying.SetActor(ctx, addr2, act2))

	// get act1 from cache
	cAct1, err := tree.GetActor(ctx, addr1)
	require.NoError(err)

	assert.Equal(uint64(1), uint64(cAct1.Nonce))
	assert.Equal(act1Cid, cAct1.Head)

	// altering act1 doesn't alter it in underlying cache
	cAct1.IncNonce()
	cAct1Cid := requireCid(t, "goodbye")
	cAct1.Head = cAct1Cid

	uAct1, err := underlying.GetActor(ctx, addr1)
	require.NoError(err)

	assert.Equal(uint64(1), uint64(uAct1.Nonce))
	assert.Equal(act1.Head, uAct1.Head)

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
	assert.Equal(cAct1Cid, uAct1Again.Head)

	// commit doesn't affect untouched actors
	uAct2, err := underlying.GetActor(ctx, addr2)
	require.NoError(err)

	assert.Equal(uint64(0), uint64(uAct2.Nonce))
	assert.Equal(act2Cid, uAct2.Head)
}

func TestCachedStateGetOrCreate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	cst := hamt.NewCborStore()
	ctx := context.Background()

	// set up state tree and cache wrapper
	underlying := NewEmptyStateTree(cst)
	tree := NewCachedStateTree(underlying)

	actorToCreate := actor.NewActor(types.AccountActorCodeCid, nil)

	// can create actor in cache
	addr := address.NewForTestGetter()()
	actor, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, error) {
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

func requireCid(t *testing.T, data string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.Raw, MhType: types.DefaultHashFunction}
	id, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return id
}
