package core

import (
	"context"
	"testing"

	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessBlockSuccess(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr1, addr2 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewTokenAmount(10000))
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
	})
	msg := types.NewMessage(addr1, addr2, types.NewTokenAmount(550), "", nil)
	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.Message{msg},
	}
	receipts, err := ProcessBlock(ctx, blk, st)
	assert.NoError(err)
	assert.Len(receipts, 1)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	expAct1, expAct2 := RequireNewAccountActor(require, types.NewTokenAmount(10000-550)), RequireNewAccountActor(require, types.NewTokenAmount(550))
	// TODO fritz when nonce checking is enforced:
	// expAct1.IncNonce()
	expStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: expAct1,
		addr2: expAct2,
	})
	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessBlockVMErrors(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	BuiltinActors[fakeActorCodeCid.KeyString()] = &FakeActor{}
	defer func() {
		delete(BuiltinActors, fakeActorCodeCid.KeyString())
	}()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := newAddress(), newAddress()
	act1, act2 := RequireNewFakeActor(require, fakeActorCodeCid), RequireNewFakeActor(require, fakeActorCodeCid)
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, nil, "returnRevertError", nil)
	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.Message{msg},
	}

	// The "foo" message will cause a vm error and
	// we're going to check four things...
	receipts, err := ProcessBlock(ctx, blk, st)

	// 1. That a VM error is not a message failure (err).
	assert.NoError(err)

	// 2. That the VM error is faithfully recorded.
	assert.Len(receipts, 1)
	assert.Contains(receipts[0].Error, "boom")

	// 3 & 4. That on VM error the state is rolled back and nonce is inc'd.
	expectedAct1, expectedAct2 := RequireNewFakeActor(require, fakeActorCodeCid), RequireNewFakeActor(require, fakeActorCodeCid)
	// TODO fritz when nonce checking is enforced:
	// expectedAct1.IncNonce()
	expectedStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: expectedAct1,
		addr2: expectedAct2,
	})
	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	assert.True(expectedStCid.Equals(gotStCid))
}

// TODO fritz add more test cases that cover the intent expressed
// in ApplyMessage's comments.
