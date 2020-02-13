package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	mh "github.com/multiformats/go-multihash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestStatePutGet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewTree(cst)

	act1 := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0))
	act1.IncrementSeqNum()
	act2 := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0))
	act2.IncrementSeqNum()
	act2.IncrementSeqNum()

	addrGetter := vmaddr.NewForTestGetter()
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

	tree2, err := NewTreeLoader().LoadStateTree(ctx, cst, tcid)
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
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewTree(cst)

	a, err := tree.GetActor(ctx, vmaddr.NewForTestGetter()())
	assert.Nil(t, a)
	assert.Error(t, err)
	assert.True(t, IsActorNotFoundError(err))

	c, err := cid.V1Builder{Codec: cid.DagCBOR, MhType: mh.BLAKE2B_MIN + 31}.Sum([]byte("cats"))
	assert.NoError(t, err)

	tr2, err := NewTreeLoader().LoadStateTree(ctx, cst, c)
	assert.Error(t, err)
	assert.Nil(t, tr2)
}

func TestStateGetOrCreate(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewTree(cst)

	addr := vmaddr.NewForTestGetter()()

	// no actor - error
	t.Run("no actor - error", func(t *testing.T) {
		actor, _, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, address.Address, error) {
			return nil, addr, fmt.Errorf("fail")
		})
		assert.EqualError(t, err, "fail")
		assert.Nil(t, actor)
	})

	t.Run("no actor - success", func(t *testing.T) {
		a, _, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, address.Address, error) {
			return &actor.Actor{}, addr, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, a, &actor.Actor{})
	})

	t.Run("actor exists", func(t *testing.T) {
		a := actor.NewActor(cid.Undef, abi.NewTokenAmount(10))
		assert.NoError(t, tree.SetActor(ctx, addr, a))

		actorBack, _, err := tree.GetOrCreateActor(ctx, addr, func() (*actor.Actor, address.Address, error) {
			return &actor.Actor{}, addr, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, actorBack, a)
	})
}

func TestGetAllActors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewTree(cst)
	addr := vmaddr.NewForTestGetter()()

	actor := actor.Actor{Code: e.NewCid(builtin.AccountActorCodeID), CallSeqNum: 1234, Balance: abi.NewTokenAmount(123)}
	err := tree.SetActor(ctx, addr, &actor)
	assert.NoError(t, err)
	_, err = tree.Flush(ctx)
	require.NoError(t, err)

	results := tree.GetAllActors(ctx)

	for result := range results {
		assert.Equal(t, addr, result.Address)
		assert.Equal(t, actor.Code, result.Actor.Code)
		assert.Equal(t, actor.CallSeqNum, result.Actor.CallSeqNum)
		assert.Equal(t, actor.Balance, result.Actor.Balance)
	}
}
