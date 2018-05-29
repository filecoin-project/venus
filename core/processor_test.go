package core

import (
	"context"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireMakeStateTree(require *require.Assertions, cst *hamt.CborIpldStore, acts map[types.Address]*types.Actor) (*cid.Cid, state.Tree) {
	ctx := context.Background()
	t := state.NewEmptyStateTreeWithActors(cst, builtin.Actors)

	for addr, act := range acts {
		err := t.SetActor(ctx, addr, act)
		require.NoError(err)
	}

	c, err := t.Flush(ctx)
	require.NoError(err)

	return c, t
}

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
	msg := types.NewMessage(addr1, addr2, 0, types.NewTokenAmount(550), "", nil)
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
	expAct1, expAct2 := RequireNewAccountActor(require, types.NewTokenAmount(10000-550)), RequireNewEmptyActor(require, types.NewTokenAmount(550))
	expAct1.IncNonce()
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
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	// Stick one empty actor and one fake actor in the state tree so they can talk.
	addr1, addr2 := newAddress(), newAddress()
	act1, act2 := RequireNewEmptyActor(require, types.NewTokenAmount(0)), RequireNewFakeActor(require, fakeActorCodeCid)
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, 0, nil, "returnRevertError", nil)
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
	assert.Contains(string(receipts[0].ReturnValue()), "boom")

	// 3 & 4. That on VM error the state is rolled back and nonce is inc'd.
	expectedAct1, expectedAct2 := RequireNewEmptyActor(require, types.NewTokenAmount(0)), RequireNewFakeActor(require, fakeActorCodeCid)
	expectedAct1.IncNonce()
	expectedStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: expectedAct1,
		addr2: expectedAct2,
	})
	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	assert.True(expectedStCid.Equals(gotStCid))
}

func TestProcessBlockParamsLengthError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewTokenAmount(1000))
	act2 := RequireNewMinerActor(require, addr1, []byte{}, types.NewBytesAmount(10000), types.NewTokenAmount(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	params, err := abi.ToValues([]interface{}{"param"})
	assert.NoError(err)
	badParams, err := abi.EncodeValues(params)
	assert.NoError(err)
	msg := types.NewMessage(addr1, addr2, 0, types.NewTokenAmount(550), "addAsk", badParams)

	r, err := ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err) // No error means definitely no fault error, which is what we're especially testing here.

	assert.Contains(string(r.ReturnValue()), "invalid params: expected 2 parameters, but got 1")
}

func TestProcessBlockParamsError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewTokenAmount(1000))
	act2 := RequireNewMinerActor(require, addr1, []byte{}, types.NewBytesAmount(10000), types.NewTokenAmount(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	badParams := []byte{1, 2, 3, 4, 5}
	msg := types.NewMessage(addr1, addr2, 0, types.NewTokenAmount(550), "addAsk", badParams)

	r, err := ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.NoError(err) // No error means definitely no fault error, which is what we're especially testing here.

	assert.Contains(string(r.ReturnValue()), "invalid params: malformed stream")
}

func TestProcessBlockNonceTooLow(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewTokenAmount(1000))
	act1.Nonce = 5
	act2 := RequireNewMinerActor(require, addr1, []byte{}, types.NewBytesAmount(10000), types.NewTokenAmount(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, 0, types.NewTokenAmount(550), "", []byte{})

	_, err := ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Equal(err.(*errors.ApplyErrorPermanent).Cause(), errNonceTooLow)
}

func TestProcessBlockNonceTooHigh(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewTokenAmount(1000))
	act2 := RequireNewMinerActor(require, addr1, []byte{}, types.NewBytesAmount(10000), types.NewTokenAmount(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, 5, types.NewTokenAmount(550), "", []byte{})

	_, err := ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Equal(err.(*errors.ApplyErrorTemporary).Cause(), errNonceTooHigh)
}

// TODO fritz add more test cases that cover the intent expressed
// in ApplyMessage's comments.

func TestNestedSendBalance(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := RequireNewAccountActor(require, types.NewTokenAmount(101))
	act1 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(102))
	act2 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(0))

	_, st := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
	params1, err := abi.ToEncodedValues(addr2)
	assert.NoError(err)
	msg1 := types.NewMessage(addr0, addr1, 0, nil, "nestedBalance", params1)

	_, err = ApplyMessage(ctx, st, msg1, types.NewBlockHeight(0))
	assert.NoError(err)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	expAct0 := RequireNewAccountActor(require, types.NewTokenAmount(101))
	expAct0.IncNonce() // because this actor has sent one message
	expAct1 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(2))
	expAct2 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(100))

	expStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr0: expAct0,
		addr1: expAct1,
		addr2: expAct2,
	})

	assert.True(expStCid.Equals(gotStCid))
}

func TestReentrantTransferDoesntAllowMultiSpending(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := RequireNewAccountActor(require, types.NewTokenAmount(0))
	act1 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(100))
	act2 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(0))

	_, st := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends twice
	params, err := abi.ToEncodedValues(addr1, addr2)
	assert.NoError(err)
	msg := types.NewMessage(addr0, addr1, 0, types.ZeroToken, "attemptMultiSpend1", params)
	_, err = ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Contains(err.Error(), "second callSendTokens")
	assert.Contains(err.Error(), "not enough balance")

	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends and then spending directly
	params, err = abi.ToEncodedValues(addr1, addr2)
	assert.NoError(err)
	msg = types.NewMessage(addr0, addr1, 0, types.ZeroToken, "attemptMultiSpend2", params)
	_, err = ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Contains(err.Error(), "failed sendTokens")
	assert.Contains(err.Error(), "not enough balance")
}

func TestSendToNonExistantAddressThenSpendFromIt(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	addr1, addr2, addr3 := newAddress(), newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewTokenAmount(1000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr1: act1,
	})

	// send 500 from addr1 to addr2
	msg := types.NewMessage(addr1, addr2, 0, types.NewTokenAmount(500), "", []byte{})
	_, err := ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	require.NoError(err)

	// send 250 along from addr2 to addr3
	msg = types.NewMessage(addr2, addr3, 0, types.NewTokenAmount(300), "", []byte{})
	_, err = ApplyMessage(ctx, st, msg, types.NewBlockHeight(0))
	require.NoError(err)

	// get all 3 actors
	act1 = state.MustGetActor(st, addr1)
	assert.Equal(types.NewTokenAmount(500), act1.Balance)
	assert.True(types.AccountActorCodeCid.Equals(act1.Code))

	act2 := state.MustGetActor(st, addr2)
	assert.Equal(types.NewTokenAmount(200), act2.Balance)
	assert.True(types.AccountActorCodeCid.Equals(act2.Code))

	act3 := state.MustGetActor(st, addr3)
	assert.Equal(types.NewTokenAmount(300), act3.Balance)
	assert.Nil(act3.Code)
}

func TestApplyQueryMessageWillNotAlterState(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := RequireNewAccountActor(require, types.NewTokenAmount(101))
	act1 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(102))
	act2 := RequireNewFakeActorWithTokens(require, fakeActorCodeCid, types.NewTokenAmount(0))

	_, st := RequireMakeStateTree(require, cst, map[types.Address]*types.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// pre-execution state
	preCid, err := st.Flush(ctx)
	require.NoError(err)

	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
	params1, err := abi.ToEncodedValues(addr2)
	assert.NoError(err)
	msg1 := types.NewMessage(addr0, addr1, 0, nil, "nestedBalance", params1)

	_, exitCode, err := ApplyQueryMessage(ctx, st, msg1, types.NewBlockHeight(0))
	require.Equal(uint8(0), exitCode)
	require.NoError(err)

	// post-execution state
	postCid, err := st.Flush(ctx)
	require.NoError(err)
	assert.True(preCid.Equals(postCid))
}
