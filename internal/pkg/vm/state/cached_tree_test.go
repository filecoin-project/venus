package state

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"

	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedStateGetCommit(t *testing.T) {
	tf.UnitTest(t)

	cst := cbor.NewMemCborStore()
	ctx := context.Background()

	// set up state tree and cache wrapper
	underlying := NewTree(cst)
	tree := NewCachedTree(underlying)

	// create some actors
	act1 := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0))
	act1Cid := requireCid(t, "hello")
	act1.Head = e.NewCid(act1Cid)
	act1.IncrementSeqNum()
	act2 := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0))
	act2Cid := requireCid(t, "world")
	act2.Head = e.NewCid(act2Cid)

	addrGetter := vmaddr.NewForTestGetter()
	addr1, addr2 := addrGetter(), addrGetter()

	// add actors to underlying cache
	assert.NoError(t, underlying.SetActor(ctx, addr1, act1))
	assert.NoError(t, underlying.SetActor(ctx, addr2, act2))

	// get act1 from cache
	cAct1, err := tree.GetActor(ctx, addr1)
	require.NoError(t, err)

	assert.Equal(t, uint64(1), cAct1.CallSeqNum)
	assert.Equal(t, act1Cid, cAct1.Head.Cid)

	// altering act1 doesn't alter it in underlying cache
	cAct1.IncrementSeqNum()
	cAct1Cid := requireCid(t, "goodbye")
	cAct1.Head = e.NewCid(cAct1Cid)

	uAct1, err := underlying.GetActor(ctx, addr1)
	require.NoError(t, err)

	assert.Equal(t, uint64(1), uAct1.CallSeqNum)
	assert.Equal(t, act1.Head.Cid, uAct1.Head.Cid)

	// retrieving from the cache again returns the same instance
	cAct1Again, err := tree.GetActor(ctx, addr1)
	require.NoError(t, err)
	assert.True(t, cAct1 == cAct1Again, "Cache returns same instance on second get")

	// commit changes sets changes in underlying tree
	err = tree.Commit(ctx)
	require.NoError(t, err)

	uAct1Again, err := underlying.GetActor(ctx, addr1)
	require.NoError(t, err)

	assert.Equal(t, uint64(2), uAct1Again.CallSeqNum)
	assert.Equal(t, cAct1Cid, uAct1Again.Head.Cid)

	// commit doesn't affect untouched actors
	uAct2, err := underlying.GetActor(ctx, addr2)
	require.NoError(t, err)

	assert.Equal(t, uint64(0), uAct2.CallSeqNum)
	assert.Equal(t, act2Cid, uAct2.Head.Cid)
}

func TestCachedStateGetOrCreate(t *testing.T) {
	tf.UnitTest(t)

	cst := cbor.NewMemCborStore()
	ctx := context.Background()

	// set up state tree and cache wrapper
	underlying := NewTree(cst)
	tree := NewCachedTree(underlying)

	actorToCreate := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0))

	// can create actor in cache
	addr := vmaddr.NewForTestGetter()()
	actor, _, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, address.Address, error) {
		return actorToCreate, addr, nil
	})
	require.NoError(t, err)
	assert.True(t, actor == actorToCreate, "GetOrCreate returns same instance created in creator")

	// cache returns same instance
	cAct, err := tree.GetActor(ctx, addr)
	require.NoError(t, err)
	assert.True(t, actor == cAct, "actor retrieved from cache is same as actor created in cache")

	// GetOrCreate does not add actor to underlying tree
	_, err = underlying.GetActor(ctx, addr)
	require.Equal(t, "actor not found", err.Error())
}

func requireCid(t *testing.T, data string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.Raw, MhType: types.DefaultHashFunction}
	id, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return id
}
