// stm: #unit
package tree

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func TestStatePutGet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	bs := repo.NewInMemoryRepo().Datastore()
	cst := cbor.NewCborStore(bs)
	tree, err := NewStateWithBuiltinActor(t, cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}

	addrGetter := testhelpers.NewForTestGetter()
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

	// stm: @STATE_VIEW_GET_ACTOR_001
	act1out, found, err := tree.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(1), act1out.Nonce)
	act2out, found, err := tree.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(2), act2out.Nonce)

	// now test it persists across recreation of tree
	tcid, err := tree.Flush(ctx)
	assert.NoError(t, err)

	tree2, err := LoadState(context.Background(), cst, tcid)
	assert.NoError(t, err)

	act1out2, found, err := tree2.GetActor(ctx, addr1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(1), act1out2.Nonce)
	act2out2, found, err := tree2.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, uint64(2), act2out2.Nonce)

	// stm: @STATE_VIEW_DELETE_ACTOR_001
	assert.NoError(t, tree2.DeleteActor(ctx, addr2))
	_, found, err = tree2.GetActor(ctx, addr2)
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestStateErrors(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := repo.NewInMemoryRepo().Datastore()
	cst := cbor.NewCborStore(bs)
	tree, err := NewStateWithBuiltinActor(t, cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}

	AddAccount(t, tree, cst, testhelpers.NewForTestGetter()())

	c, err := constants.DefaultCidBuilder.Sum([]byte("cats"))
	assert.NoError(t, err)

	// stm: @STATE_TREE_LOAD_STATE_001
	tr2, err := LoadState(ctx, cst, c)
	assert.Error(t, err)
	assert.Nil(t, tr2)
}

func TestGetAllActors(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := repo.NewInMemoryRepo().Datastore()
	cst := cbor.NewCborStore(bs)
	tree, err := NewStateWithBuiltinActor(t, cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}
	addr := testhelpers.NewForTestGetter()()

	newActor := types.Actor{Code: builtin2.AccountActorCodeID, Nonce: 1234, Balance: abi.NewTokenAmount(123)}
	AddAccount(t, tree, cst, addr)
	// stm: @STATE_VIEW_FLUSH_001
	_, err = tree.Flush(ctx)
	require.NoError(t, err)

	// stm: @STATE_VIEW_FOR_EACH_001
	err = tree.ForEach(func(key ActorKey, result *types.Actor) error {
		if addr != key {
			return nil
		}
		assert.Equal(t, addr, key)
		assert.Equal(t, newActor.Code, result.Code)
		assert.Equal(t, newActor.Nonce, result.Nonce)
		assert.Equal(t, newActor.Balance, result.Balance)
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

func TestSnapshot(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := repo.NewInMemoryRepo().Datastore()
	cst := cbor.NewCborStore(bs)
	tree, err := NewState(cst, StateTreeVersion1)
	if err != nil {
		t.Fatal(err)
	}

	// stm: @STATE_VIEW_SNAPSHOT_001
	assert.NoError(t, tree.Snapshot(ctx))
	assert.Equal(t, len(tree.snaps.layers), 2)

	randomCid, err := cid.Decode("bafy2bzacecu7n7wbtogznrtuuvf73dsz7wasgyneqasksdblxupnyovmtwxxu")
	if err != nil {
		t.Fatal(err)
	}

	addr, err := address.NewIDAddress(uint64(1007))
	assert.NoError(t, err)

	actor := &types.Actor{
		Code:    randomCid,
		Head:    randomCid,
		Balance: abi.NewTokenAmount(int64(10000)),
		Nonce:   100,
	}

	if err := tree.SetActor(ctx, addr, actor); err != nil {
		t.Fatal(err)
	}

	resActor, find, err := tree.GetActor(ctx, addr)
	assert.NoError(t, err)
	assert.True(t, find)
	assert.Equal(t, actor, resActor)

	// stm: @STATE_VIEW_REVERT_001
	assert.NoError(t, tree.Revert())

	resActor, find, err = tree.GetActor(ctx, addr)
	assert.NoError(t, err)
	assert.Nil(t, resActor)
	assert.False(t, find)

	// stm: @STATE_VIEW_CLEAR_SNAPSHOT_001
	tree.ClearSnapshot()
	assert.Equal(t, len(tree.snaps.layers), 1)
}

func TestStateTreeConsistency(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	bs := repo.NewInMemoryRepo().Datastore()
	cst := cbor.NewCborStore(bs)
	// stm: @STATE_TREE_NEW_STATE_001
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
		// stm: @STATE_VIEW_SET_ACTOR_001, @STATE_VIEW_LOOKUP_ID_001
		if err := tree.SetActor(ctx, a, &types.Actor{
			Code:    randomCid,
			Head:    randomCid,
			Balance: abi.NewTokenAmount(int64(10000 + i)),
			Nonce:   uint64(1000 - i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	root, err := tree.Flush(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if root.String() != "bafy2bzaceamis23jp44ofm4fh6jwc4gkxlzhnvxrdw4zsn3v2fj6at6pf2m4y" {
		t.Fatalf("state state Mismatch. Expected: bafy2bzaceamis23jp44ofm4fh6jwc4gkxlzhnvxrdw4zsn3v2fj6at6pf2m4y Actual: %s", root.String())
	}
}
