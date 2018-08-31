package core

import (
	"context"
	"testing"

	hamt "gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireMakeStateTree(require *require.Assertions, cst *hamt.CborIpldStore, acts map[types.Address]*actor.Actor) (*cid.Cid, state.Tree) {
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

	toAddr := newAddress()
	fromAddr := mockSigner.Addresses[0] // fromAddr needs to be known by signer
	fromAct := RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr: fromAct,
	})

	vms := VMStorage()
	msg := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(550), "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner)
	require.NoError(err)

	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg},
	}
	results, err := ProcessBlock(ctx, blk, st, vms)
	assert.NoError(err)
	assert.Len(results, 1)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	expAct1, expAct2 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000-550)), RequireNewEmptyActor(require, types.NewAttoFILFromFIL(550))
	expAct1.IncNonce()
	expStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr: expAct1,
		toAddr:   expAct2,
	})
	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessTipSetSuccess(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	toAddr := newAddress()
	fromAddr1 := mockSigner.Addresses[0]
	fromAddr2 := mockSigner.Addresses[1]

	fromAddr1Act := RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	fromAddr2Act := RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr1: fromAddr1Act,
		fromAddr2: fromAddr2Act,
	})

	msg1 := types.NewMessage(fromAddr1, toAddr, 0, types.NewAttoFILFromFIL(550), "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner)
	require.NoError(err)
	blk1 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg1},
	}

	msg2 := types.NewMessage(fromAddr2, toAddr, 0, types.NewAttoFILFromFIL(50), "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner)
	require.NoError(err)
	blk2 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg2},
	}

	res, err := ProcessTipSet(ctx, RequireNewTipSet(require, blk1, blk2), st, vms)
	assert.NoError(err)
	assert.Len(res.Results, 2)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	expAct1, expAct2, expAct3 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000-550)), RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000-50)), RequireNewEmptyActor(require, types.NewAttoFILFromFIL(550+50))
	expAct1.IncNonce()
	expAct2.IncNonce()
	expStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr1: expAct1,
		fromAddr2: expAct2,
		toAddr:    expAct3,
	})
	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessTipsConflicts(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	fromAddr, toAddr := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr: act1,
	})

	msg1 := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(501), "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner)
	require.NoError(err)
	blk1 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg1},
		Ticket:    []byte{0, 0}, // Block with smaller ticket
	}

	msg2 := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(502), "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner)
	require.NoError(err)
	blk2 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg2},
		Ticket:    []byte{1, 1},
	}
	res, err := ProcessTipSet(ctx, RequireNewTipSet(require, blk1, blk2), st, vms)
	assert.NoError(err)
	assert.Len(res.Results, 1)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	expAct1, expAct2 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000-501)), RequireNewEmptyActor(require, types.NewAttoFILFromFIL(501))
	expAct1.IncNonce()
	expStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr: expAct1,
		toAddr:   expAct2,
	})
	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessBlockVMErrors(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	// Stick one empty actor and one fake actor in the state tree so they can talk.
	fromAddr, toAddr := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1, act2 := RequireNewEmptyActor(require, types.NewAttoFILFromFIL(0)), RequireNewFakeActor(require, vms, toAddr, fakeActorCodeCid)
	stCid, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr: act1,
		toAddr:   act2,
	})
	msg := types.NewMessage(fromAddr, toAddr, 0, nil, "returnRevertError", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner)
	require.NoError(err)
	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg},
	}

	// The "foo" message will cause a vm error and
	// we're going to check four things...
	results, err := ProcessBlock(ctx, blk, st, vms)

	// 1. That a VM error is not a message failure (err).
	assert.NoError(err)

	// 2. That the VM error is faithfully recorded.
	assert.Len(results, 1)
	assert.Len(results[0].Receipt.Return, 0)
	assert.Contains(results[0].ExecutionError.Error(), "boom")

	// 3 & 4. That on VM error the state is rolled back and nonce is inc'd.
	expectedAct1, expectedAct2 := RequireNewEmptyActor(require, types.NewAttoFILFromFIL(0)), RequireNewFakeActor(require, vms, toAddr, fakeActorCodeCid)
	expectedAct1.IncNonce()
	expectedStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		fromAddr: expectedAct1,
		toAddr:   expectedAct2,
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
	vms := VMStorage()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	act2 := RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, types.NewBytesAmount(10000), RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	params, err := abi.ToValues([]interface{}{"param"})
	assert.NoError(err)
	badParams, err := abi.EncodeValues(params)
	assert.NoError(err)
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "addAsk", badParams)

	rct, err := ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err) // No error means definitely no fault error, which is what we're especially testing here.

	assert.Empty(rct.Receipt.Return)
	assert.Contains(rct.ExecutionError.Error(), "invalid params: expected 2 parameters, but got 1")
}

func TestProcessBlockParamsError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	act2 := RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, types.NewBytesAmount(10000), RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	badParams := []byte{1, 2, 3, 4, 5}
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "addAsk", badParams)

	rct, err := ApplyMessage(ctx, st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err) // No error means definitely no fault error, which is what we're especially testing here.

	assert.Empty(rct.Receipt.Return)
	assert.Contains(rct.ExecutionError.Error(), "invalid params: malformed stream")
}

func TestProcessBlockNonceTooLow(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	act1.Nonce = 5
	act2 := RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, types.NewBytesAmount(10000), RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "", []byte{})

	_, err := ApplyMessage(ctx, st, VMStorage(), msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Equal(err.(*errors.ApplyErrorPermanent).Cause(), errNonceTooLow)
}

func TestProcessBlockNonceTooHigh(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	addr2, addr1 := newAddress(), newAddress()
	act1 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	act2 := RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, types.NewBytesAmount(10000), RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	msg := types.NewMessage(addr1, addr2, 5, types.NewAttoFILFromFIL(550), "", []byte{})

	_, err := ApplyMessage(ctx, st, VMStorage(), msg, types.NewBlockHeight(0))
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
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(101))
	act1 := RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
	act2 := RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

	_, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
	params1, err := abi.ToEncodedValues(addr2)
	assert.NoError(err)
	msg1 := types.NewMessage(addr0, addr1, 0, nil, "nestedBalance", params1)

	_, err = ApplyMessage(ctx, st, VMStorage(), msg1, types.NewBlockHeight(0))
	assert.NoError(err)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	expAct0 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(101))
	expAct0.IncNonce() // because this actor has sent one message
	expAct1 := RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(2))
	expAct2 := RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(100))

	expStCid, _ := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
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
	vms := VMStorage()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
	act1 := RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(100))
	act2 := RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

	_, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends twice
	params, err := abi.ToEncodedValues(addr1, addr2)
	assert.NoError(err)
	msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "attemptMultiSpend1", params)
	_, err = ApplyMessage(ctx, st, VMStorage(), msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Contains(err.Error(), "second callSendTokens")
	assert.Contains(err.Error(), "not enough balance")

	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends and then spending directly
	params, err = abi.ToEncodedValues(addr1, addr2)
	assert.NoError(err)
	msg = types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "attemptMultiSpend2", params)
	_, err = ApplyMessage(ctx, st, VMStorage(), msg, types.NewBlockHeight(0))
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
	act1 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	_, st := requireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr1: act1,
	})

	// send 500 from addr1 to addr2
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(500), "", []byte{})
	_, err := ApplyMessage(ctx, st, VMStorage(), msg, types.NewBlockHeight(0))
	require.NoError(err)

	// send 250 along from addr2 to addr3
	msg = types.NewMessage(addr2, addr3, 0, types.NewAttoFILFromFIL(300), "", []byte{})
	_, err = ApplyMessage(ctx, st, VMStorage(), msg, types.NewBlockHeight(0))
	require.NoError(err)

	// get all 3 actors
	act1 = state.MustGetActor(st, addr1)
	assert.Equal(types.NewAttoFILFromFIL(500), act1.Balance)
	assert.True(types.AccountActorCodeCid.Equals(act1.Code))

	act2 := state.MustGetActor(st, addr2)
	assert.Equal(types.NewAttoFILFromFIL(200), act2.Balance)
	assert.True(types.AccountActorCodeCid.Equals(act2.Code))

	act3 := state.MustGetActor(st, addr3)
	assert.Equal(types.NewAttoFILFromFIL(300), act3.Balance)
	assert.Nil(act3.Code)
}

func TestApplyQueryMessageWillNotAlterState(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := types.NewAddressForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := VMStorage()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid.KeyString()] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid.KeyString())
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := RequireNewAccountActor(require, types.NewAttoFILFromFIL(101))
	act1 := RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
	act2 := RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

	_, st := RequireMakeStateTree(require, cst, map[types.Address]*actor.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// pre-execution state
	preCid, err := st.Flush(ctx)
	require.NoError(err)

	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
	args1, err := abi.ToEncodedValues(addr2)
	assert.NoError(err)

	_, exitCode, err := CallQueryMethod(ctx, st, vms, addr1, "nestedBalance", args1, addr0, types.NewBlockHeight(0))
	require.Equal(uint8(0), exitCode)
	require.NoError(err)

	// post-execution state
	postCid, err := st.Flush(ctx)
	require.NoError(err)
	assert.True(preCid.Equals(postCid))
}
