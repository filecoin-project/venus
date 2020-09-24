package state

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/ipfs/go-cid"
)

func TestStatePutGet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewState(cst)

	act1 := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0), cid.Undef)
	act1.IncrementSeqNum()
	act2 := actor.NewActor(builtin.AccountActorCodeID, abi.NewTokenAmount(0), cid.Undef)
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
	tcid, err := tree.Flush(ctx)
	assert.NoError(t, err)

	tree2, err := LoadState(context.Background(), cst, tcid)
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

	newActor := actor.Actor{Code: builtin.AccountActorCodeID, CallSeqNum: 1234, Balance: abi.NewTokenAmount(123)}
	err := tree.SetActor(ctx, addr, &newActor)
	assert.NoError(t, err)
	_, err = tree.Flush(ctx)
	require.NoError(t, err)

	err = tree.ForEach(func(key ActorKey, result *actor.Actor) error {
		assert.Equal(t, addr, key)
		assert.Equal(t, newActor.Code, result.Code)
		assert.Equal(t, newActor.CallSeqNum, result.CallSeqNum)
		assert.Equal(t, newActor.Balance, result.Balance)
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

func TestStateTreeConsistency(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree := NewState(cst)

	var addrs []address.Address
	for i := 100; i < 150; i++ {
		a, err := address.NewIDAddress(uint64(i))
		if err != nil {
			t.Fatal(err)
		}

		addrs = append(addrs, a)
	}

	randomCid, err := cid.Decode("bafy2bzacecu7n7wbtogznrtuuvf73dsz7wasgyneqasksdblxupnyovmtwxxu")
	if err != nil {
		t.Fatal(err)
	}

	for i, a := range addrs {
		if err := tree.SetActor(ctx, a, &actor.Actor{
			Code:       randomCid,
			Head:       randomCid,
			Balance:    abi.NewTokenAmount(int64(10000 + i)),
			CallSeqNum: uint64(1000 - i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	root, err := tree.Flush(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if root.String() != "bafy2bzaceadyjnrv3sbjvowfl3jr4pdn5p2bf3exjjie2f3shg4oy5sub7h34" {
		t.Fatalf("State State Mismatch. Expected: bafy2bzaceadyjnrv3sbjvowfl3jr4pdn5p2bf3exjjie2f3shg4oy5sub7h34 Actual: %s", root.String())
	}

}
