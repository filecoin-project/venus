package state

import (
	"context"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
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
	tree := NewState(cst)

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

	act1out, found, err := tree.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, act1, act1out)
	act2out, found, err := tree.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, act2, act2out)

	// now test it persists across recreation of tree
	tcid, err := tree.Commit(ctx)
	assert.NoError(t, err)

	tree2, err := LoadState(ctx, cst, tcid)
	assert.NoError(t, err)

	act1out2, found, err := tree2.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, act1, act1out2)
	act2out2, found, err := tree2.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, act2, act2out2)
}

func TestStateErrors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewState(cst)

	a, found, err := tree.GetActor(ctx, vmaddr.NewForTestGetter()())
	assert.Nil(t, a)
	assert.False(t, found)
	assert.NoError(t, err)

	c, err := constants.DefaultCidBuilder.Sum([]byte("cats"))
	assert.NoError(t, err)

	tr2, err := LoadState(ctx, cst, c)
	assert.Error(t, err)
	assert.Nil(t, tr2)
}
func TestGetAllActors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewState(cst)
	addr := vmaddr.NewForTestGetter()()

	actor := actor.Actor{Code: e.NewCid(builtin.AccountActorCodeID), CallSeqNum: 1234, Balance: abi.NewTokenAmount(123)}
	err := tree.SetActor(ctx, addr, &actor)
	assert.NoError(t, err)
	_, err = tree.Commit(ctx)
	require.NoError(t, err)

	results := tree.GetAllActors(ctx)

	for result := range results {
		assert.Equal(t, addr, result.Key)
		assert.Equal(t, actor.Code, result.Actor.Code)
		assert.Equal(t, actor.CallSeqNum, result.Actor.CallSeqNum)
		assert.Equal(t, actor.Balance, result.Actor.Balance)
	}
}
