package state

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/cborutil"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func TestStatePutGet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree, err := NewStateWithBuiltinActor(t, cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}

	addrGetter := types.NewForTestGetter()
	addr1 := addrGetter()
	addr2 := addrGetter()
	AddAccount(t, tree, cst, addr1)
	AddAccount(t, tree, cst, addr2)

	UpdateAccount(t, tree, addr1, func(act1 *types.Actor) {
		act1.IncrementSeqNum()
	})

	UpdateAccount(t, tree, addr2, func(act2 *types.Actor) {
		act2.IncrementSeqNum()
		act2.IncrementSeqNum()
	})

	act1out, found, err := tree.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(1), act1out.CallSeqNum)
	act2out, found, err := tree.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(2), act2out.CallSeqNum)

	// now test it persists across recreation of tree
	tcid, err := tree.Flush(ctx)
	assert.NoError(t, err)

	tree2, err := LoadState(context.Background(), cst, tcid)
	assert.NoError(t, err)

	act1out2, found, err := tree2.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(1), act1out2.CallSeqNum)
	act2out2, found, err := tree2.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(2), act2out2.CallSeqNum)
}

func TestStateErrors(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := cborutil.NewIpldStore(bs)
	tree, err := NewStateWithBuiltinActor(t, cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}

	AddAccount(t, tree, cst, types.NewForTestGetter()())

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
	tree, err := NewStateWithBuiltinActor(t, cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}
	addr := types.NewForTestGetter()()

	newActor := types.Actor{Code: enccid.NewCid(builtin2.AccountActorCodeID), CallSeqNum: 1234, Balance: abi.NewTokenAmount(123)}
	AddAccount(t, tree, cst, addr)
	_, err = tree.Flush(ctx)
	require.NoError(t, err)

	err = tree.ForEach(func(key ActorKey, result *types.Actor) error {
		if addr != key {
			return nil
		}
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
	tree, err := NewState(cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}

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
		if err := tree.SetActor(ctx, a, &types.Actor{
			Code:       enccid.NewCid(randomCid),
			Head:       enccid.NewCid(randomCid),
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
	if root.String() != "bafy2bzacebzirvsnrqtjkzpc3md2lql46owuqcshwtpxv4hrbzead6xj5ywrk" {
		t.Fatalf("state state Mismatch. Expected: bafy2bzaceb2bhqw75pqp44efoxvlnm73lnctq6djair56bfn5x3gw56epcxbi Actual: %s", root.String())
	}

}
