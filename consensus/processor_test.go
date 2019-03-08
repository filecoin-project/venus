package consensus_test

import (
	"context"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	. "github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

func requireMakeStateTree(require *require.Assertions, cst *hamt.CborIpldStore, acts map[address.Address]*actor.Actor) (cid.Cid, state.Tree) {
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

	newAddress := address.NewForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

	startingNetworkBalance := uint64(10000000)

	toAddr := newAddress()
	minerAddr := newAddress()
	minerOwnerAddr := newAddress()
	fromAddr := mockSigner.Addresses[0] // fromAddr needs to be known by signer
	fromAct := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	vms := th.VMStorage()
	minerActor := th.RequireNewMinerActor(require, vms, minerAddr, minerOwnerAddr, []byte{}, 10, th.RequireRandomPeerID(), types.ZeroAttoFIL)
	stCid, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(startingNetworkBalance)),
		minerAddr:              minerActor,
		minerOwnerAddr:         th.RequireNewAccountActor(require, types.ZeroAttoFIL),
		fromAddr:               fromAct,
	})

	msg := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(550), "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg},
		Miner:     minerAddr,
	}
	results, err := NewDefaultProcessor().ProcessBlock(ctx, st, vms, blk, nil)
	assert.NoError(err)
	assert.Len(results, 1)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	expAct1, expAct2 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000-550)), th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(550))
	expAct1.IncNonce()
	blockRewardAmount := NewDefaultBlockRewarder().BlockRewardAmount()
	expectedNetworkBalance := types.NewAttoFILFromFIL(startingNetworkBalance).Sub(blockRewardAmount)
	expStCid, _ := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, expectedNetworkBalance),
		minerAddr:              minerActor,
		minerOwnerAddr:         th.RequireNewAccountActor(require, blockRewardAmount),
		fromAddr:               expAct1,
		toAddr:                 expAct2,
	})

	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessTipSetSuccess(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	newAddress := address.NewForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	startingNetworkBalance := types.NewAttoFILFromFIL(1000000)
	minerAddr := newAddress()

	toAddr := newAddress()
	mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

	fromAddr1 := mockSigner.Addresses[0]
	fromAddr2 := mockSigner.Addresses[1]

	fromAddr1Act := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	fromAddr2Act := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, startingNetworkBalance),
		fromAddr1:              fromAddr1Act,
		fromAddr2:              fromAddr2Act,
	})

	vms := th.VMStorage()
	minerOwner, err := address.NewActorAddress([]byte("mo"))
	require.NoError(err)
	stCid, miner := mustCreateMiner(ctx, require, st, vms, minerAddr, minerOwner)

	msg1 := types.NewMessage(fromAddr1, toAddr, 0, types.NewAttoFILFromFIL(550), "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	blk1 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg1},
		Miner:     minerAddr,
	}

	msg2 := types.NewMessage(fromAddr2, toAddr, 0, types.NewAttoFILFromFIL(50), "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	blk2 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg2},
		Miner:     minerAddr,
	}

	res, err := NewDefaultProcessor().ProcessTipSet(ctx, st, vms, th.RequireNewTipSet(require, blk1, blk2), nil)
	assert.NoError(err)
	assert.Len(res.Results, 2)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)
	expAct1, expAct2, expAct3 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000-550)), th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000-50)), th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(550+50))
	expAct1.IncNonce()
	expAct2.IncNonce()

	blockRewardAmount := NewDefaultBlockRewarder().BlockRewardAmount()
	twoBlockRewards := blockRewardAmount.Add(blockRewardAmount)
	expectedNetworkBalance := startingNetworkBalance.Sub(twoBlockRewards)
	expStCid, _ := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, expectedNetworkBalance),
		minerAddr:              miner,
		minerOwner:             th.RequireNewEmptyActor(require, twoBlockRewards),
		fromAddr1:              expAct1,
		fromAddr2:              expAct2,
		toAddr:                 expAct3,
	})
	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessTipsConflicts(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	newAddress := address.NewForTestGetter()
	startingNetworkBalance := types.NewAttoFILFromFIL(1000000)
	minerAddr := newAddress()

	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := th.VMStorage()
	mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

	fromAddr, toAddr := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, startingNetworkBalance),
		fromAddr:               act1,
	})

	minerOwner, err := address.NewActorAddress([]byte("mo"))
	require.NoError(err)
	stCid, miner := mustCreateMiner(ctx, require, st, vms, minerAddr, minerOwner)

	msg1 := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(501), "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	blk1 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg1},
		Ticket:    []byte{0, 0}, // Block with smaller ticket
		Miner:     minerAddr,
	}

	msg2 := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(502), "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	blk2 := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg2},
		Ticket:    []byte{1, 1},
		Miner:     minerAddr,
	}
	res, err := NewDefaultProcessor().ProcessTipSet(ctx, st, vms, th.RequireNewTipSet(require, blk1, blk2), nil)
	assert.NoError(err)
	assert.Len(res.Results, 1)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	expAct1, expAct2 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000-501)), th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(501))
	expAct1.IncNonce()
	blockReward := NewDefaultBlockRewarder().BlockRewardAmount()
	twoBlockRewards := blockReward.Add(blockReward)
	expectedNetworkBalance := startingNetworkBalance.Sub(twoBlockRewards)
	expStCid, _ := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, expectedNetworkBalance),
		minerOwner:             th.RequireNewEmptyActor(require, twoBlockRewards),
		minerAddr:              miner,
		fromAddr:               expAct1,
		toAddr:                 expAct2,
	})
	assert.True(expStCid.Equals(gotStCid))
}

func TestProcessBlockBadMsgSig(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	newAddress := address.NewForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

	toAddr := newAddress()
	fromAddr := mockSigner.Addresses[0] // fromAddr needs to be known by signer
	fromAct := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000))
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(100000)),
		fromAddr:               fromAct,
	})

	vms := th.VMStorage()
	minerAddr, err := address.NewActorAddress([]byte("miner"))
	require.NoError(err)
	minerOwner, err := address.NewActorAddress([]byte("mo"))
	require.NoError(err)
	stCid, _ := mustCreateMiner(ctx, require, st, vms, minerAddr, minerOwner)

	msg := types.NewMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(550), "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	// corrupt the message data
	smsg.Message.Nonce = 13

	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Miner:     minerAddr,
		Messages:  []*types.SignedMessage{smsg},
	}
	results, err := NewDefaultProcessor().ProcessBlock(ctx, st, vms, blk, nil)
	require.Nil(results)
	assert.EqualError(err, "apply message failed: invalid signature by sender over message data")
}

// ProcessBlock should not fail with an unsigned block reward message.
func TestProcessBlockReward(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	newAddress := address.NewForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()

	minerAddr := newAddress()
	minerOwnerAddr := newAddress()
	minerBalance := types.NewAttoFILFromFIL(10000)
	ownerAct := th.RequireNewAccountActor(require, minerBalance)
	networkAct := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(100000000000))
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		minerOwnerAddr: ownerAct,
		// TODO: get rid of this ugly hack as soon once we have
		// sustainable reward support (i.e. anything but setting
		// up network address in genesis with a bunch of FIL).
		address.NetworkAddress: networkAct,
	})

	vms := th.VMStorage()
	stCid, _ := mustCreateMiner(ctx, require, st, vms, minerAddr, minerOwnerAddr)

	blk := &types.Block{
		Miner:     minerAddr,
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{},
	}
	ret, err := NewDefaultProcessor().ProcessBlock(ctx, st, vms, blk, nil)
	require.NoError(err)
	assert.Nil(ret)

	minerOwnerActor, err := st.GetActor(ctx, minerOwnerAddr)
	require.NoError(err)

	blockRewardAmount := NewDefaultBlockRewarder().BlockRewardAmount()
	assert.Equal(minerBalance.Add(blockRewardAmount), minerOwnerActor.Balance)
}

func TestProcessBlockVMErrors(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := th.VMStorage()

	newAddress := address.NewForTestGetter()
	startingNetworkBalance := types.NewAttoFILFromFIL(1000000)
	minerAddr, minerOwnerAddr := newAddress(), newAddress()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid)
	}()
	mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

	// Stick one empty actor and one fake actor in the state tree so they can talk.
	fromAddr, toAddr := mockSigner.Addresses[0], mockSigner.Addresses[1]

	act1, act2 := th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(0)), th.RequireNewFakeActor(require, vms, toAddr, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, startingNetworkBalance),
		fromAddr:               act1,
		toAddr:                 act2,
	})

	stCid, miner := mustCreateMiner(ctx, require, st, vms, minerAddr, minerOwnerAddr)

	msg := types.NewMessage(fromAddr, toAddr, 0, nil, "returnRevertError", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	blk := &types.Block{
		Height:    20,
		StateRoot: stCid,
		Messages:  []*types.SignedMessage{smsg},
		Miner:     minerAddr,
	}

	// The "foo" message will cause a vm error and
	// we're going to check four things...
	results, err := NewDefaultProcessor().ProcessBlock(ctx, st, vms, blk, nil)

	// 1. That a VM error is not a message failure (err).
	assert.NoError(err)

	// 2. That the VM error is faithfully recorded.
	assert.Len(results, 1)
	assert.Len(results[0].Receipt.Return, 0)
	assert.Contains(results[0].ExecutionError.Error(), "boom")

	// 3 & 4. That on VM error the state is rolled back and nonce is inc'd.
	expectedAct1, expectedAct2 := th.RequireNewEmptyActor(require, types.NewAttoFILFromFIL(0)), th.RequireNewFakeActor(require, vms, toAddr, fakeActorCodeCid)
	expectedAct1.IncNonce()
	blockRewardAmount := NewDefaultBlockRewarder().BlockRewardAmount()
	expectedStCid, _ := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, startingNetworkBalance.Sub(blockRewardAmount)),
		minerOwnerAddr:         th.RequireNewEmptyActor(require, blockRewardAmount),
		minerAddr:              miner,
		fromAddr:               expectedAct1,
		toAddr:                 expectedAct2,
	})
	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	assert.True(expectedStCid.Equals(gotStCid))
}

func TestProcessBlockParamsLengthError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := address.NewForTestGetter()
	cst := hamt.NewCborStore()
	vms := th.VMStorage()

	addr2, addr1 := newAddress(), newAddress()
	act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, uint64(10), th.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	params, err := abi.ToValues([]interface{}{"param"})
	assert.NoError(err)
	badParams, err := abi.EncodeValues(params)
	assert.NoError(err)
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "getPower", badParams)

	rct, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err) // No error means definitely no fault error, which is what we're especially testing here.

	assert.Empty(rct.Receipt.Return)
	assert.Contains(rct.ExecutionError.Error(), "invalid params: expected 0 parameters, but got 1")
}

func TestProcessBlockParamsError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := address.NewForTestGetter()
	cst := hamt.NewCborStore()
	vms := th.VMStorage()

	addr2, addr1 := newAddress(), newAddress()
	act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, uint64(10), th.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	badParams := []byte{1, 2, 3, 4, 5}
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "getPower", badParams)

	rct, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
	assert.NoError(err) // No error means definitely no fault error, which is what we're especially testing here.

	assert.Empty(rct.Receipt.Return)
	assert.Contains(rct.ExecutionError.Error(), "invalid params: malformed stream")
}

func TestApplyMessagesValidation(t *testing.T) {
	t.Run("Errors when nonce too high", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		newAddress := address.NewForTestGetter()
		ctx := context.Background()
		cst := hamt.NewCborStore()
		vms := th.VMStorage()
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		addr1 := mockSigner.Addresses[0]
		addr2 := newAddress()
		act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
		act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, 10, th.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
		_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr1: act1,
			addr2: act2,
		})
		msg := types.NewMessage(addr1, addr2, 5, types.NewAttoFILFromFIL(550), "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
		require.NoError(err)

		_, err = NewDefaultProcessor().ApplyMessage(ctx, st, th.VMStorage(), smsg, addr2, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		assert.Error(err)
		assert.Equal("nonce too high", err.(*errors.ApplyErrorTemporary).Cause().Error())
	})

	t.Run("Errors when nonce too low", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		newAddress := address.NewForTestGetter()
		ctx := context.Background()
		cst := hamt.NewCborStore()
		vms := th.VMStorage()
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		addr1 := mockSigner.Addresses[0]
		addr2 := newAddress()
		act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
		act1.Nonce = 5
		act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, uint64(10), th.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
		_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr1: act1,
			addr2: act2,
		})
		msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
		require.NoError(err)

		_, err = NewDefaultProcessor().ApplyMessage(ctx, st, th.VMStorage(), smsg, addr2, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		assert.Error(err)
		assert.Equal("nonce too low", err.(*errors.ApplyErrorPermanent).Cause().Error())
	})

	t.Run("errors when specifying a gas limit in excess of balance", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		addr1, _, addr2, _, st, mockSigner := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
		msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, *types.NewAttoFILFromFIL(10), types.NewGasUnits(50))
		require.NoError(err)

		// the maximum gas charge (10*50 = 500) is greater than the sender balance minus the message value (1000-550 = 450)
		_, err = NewDefaultProcessor().ApplyMessage(context.Background(), st, th.VMStorage(), smsg, addr2, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		require.Error(err)
		assert.Equal("balance insufficient to cover transfer+gas", err.(*errors.ApplyErrorPermanent).Cause().Error())
	})

	t.Run("errors when sender is not an account actor", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		_, _, addr2, _, st, mockSigner := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
		addr1 := mockSigner.Addresses[0]
		act1 := th.RequireNewFakeActor(require, th.VMStorage(), addr1, types.NewCidForTestGetter()())

		ctx := context.Background()
		err := st.SetActor(ctx, addr1, act1)
		require.NoError(err)

		msg := types.NewMessage(addr1, addr2, 0, types.ZeroAttoFIL, "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, *types.NewAttoFILFromFIL(10), types.NewGasUnits(50))
		require.NoError(err)

		_, err = NewDefaultProcessor().ApplyMessage(context.Background(), st, th.VMStorage(), smsg, addr2, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		require.Error(err)
		assert.Equal("message from non-account actor", err.(*errors.ApplyErrorPermanent).Cause().Error())
	})

	t.Run("errors when sender is not an actor", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		cst := hamt.NewCborStore()
		vms := th.VMStorage()
		mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

		addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
		act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{},
			10, th.RequireRandomPeerID(), types.NewAttoFILFromFIL(1000))

		_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{addr2: act2})

		msg := types.NewMessage(addr1, addr2, 0, types.ZeroAttoFIL, "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, *types.NewAttoFILFromFIL(10), types.NewGasUnits(50))
		require.NoError(err)

		_, err = NewDefaultProcessor().ApplyMessage(context.Background(), st, th.VMStorage(), smsg, addr2,
			types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		require.Error(err)
		assert.Equal("from (sender) account not found", err.(*errors.ApplyErrorTemporary).Cause().Error())
	})

	t.Run("errors on attempt to transfer negative value", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		newAddress := address.NewForTestGetter()
		ctx := context.Background()
		cst := hamt.NewCborStore()
		vms := th.VMStorage()
		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

		addr1 := mockSigner.Addresses[0]
		addr2 := newAddress()
		act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
		act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, 10, th.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
		_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
			addr1: act1,
			addr2: act2,
		})

		someval, ok := types.NewAttoFILFromString("-500", 10)
		require.True(ok)

		msg := types.NewMessage(addr1, addr2, 0, someval, "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
		require.NoError(err)

		_, err = NewDefaultProcessor().ApplyMessage(ctx, st, th.VMStorage(), smsg, addr2, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		assert.Error(err)
		assert.Contains("negative value", err.(*errors.ApplyErrorPermanent).Cause().Error())
	})

	t.Run("errors when attempting to send to self", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		addr1, _, addr2, _, st, mockSigner := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
		msg := types.NewMessage(addr1, addr1, 0, types.NewAttoFILFromFIL(550), "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, *types.NewAttoFILFromFIL(10), types.NewGasUnits(0))
		require.NoError(err)

		// the maximum gas charge (10*50 = 500) is greater than the sender balance minus the message value (1000-550 = 450)
		_, err = NewDefaultProcessor().ApplyMessage(context.Background(), st, th.VMStorage(), smsg, addr2, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		require.Error(err)
		assert.Equal("cannot send to self", err.(*errors.ApplyErrorPermanent).Cause().Error())
	})

	t.Run("errors when specifying a gas limit in excess of balance", func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		addr1, _, addr2, _, st, mockSigner := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
		msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), "", []byte{})
		smsg, err := types.NewSignedMessage(*msg, mockSigner, *types.NewAttoFILFromFIL(10), types.NewGasUnits(50))
		require.NoError(err)

		// the maximum gas charge (10*50 = 500) is greater than the sender balance minus the message value (1000-550 = 450)
		_, err = NewDefaultProcessor().ApplyMessage(context.Background(), st, th.VMStorage(), smsg, address.Undef, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
		require.Error(err)
		assert.Equal("balance insufficient to cover transfer+gas", err.(*errors.ApplyErrorPermanent).Cause().Error())
	})
}

// TODO add more test cases that cover the intent expressed
// in ApplyMessage's comments.

func TestNestedSendBalance(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := address.NewForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
	vms := vm.NewStorageMap(bs)

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid)
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(101))
	act1 := th.RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
	act2 := th.RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
	params1, err := abi.ToEncodedValues(addr2)
	assert.NoError(err)
	msg1 := types.NewMessage(addr0, addr1, 0, nil, "nestedBalance", params1)

	_, err = th.ApplyTestMessage(st, th.VMStorage(), msg1, types.NewBlockHeight(0))
	assert.NoError(err)

	gotStCid, err := st.Flush(ctx)
	assert.NoError(err)

	expAct0 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(101))
	expAct0.Nonce = 1
	expAct1 := th.RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(2))
	expAct2 := th.RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(100))

	expStCid, _ := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr0: expAct0,
		addr1: expAct1,
		addr2: expAct2,
	})

	assert.True(expStCid.Equals(gotStCid))
}

func TestReentrantTransferDoesntAllowMultiSpending(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := address.NewForTestGetter()
	cst := hamt.NewCborStore()
	vms := th.VMStorage()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid)
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0))
	act1 := th.RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(100))
	act2 := th.RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr0: act0,
		addr1: act1,
		addr2: act2,
	})

	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends twice
	params, err := abi.ToEncodedValues(addr1, addr2)
	assert.NoError(err)
	msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "attemptMultiSpend1", params)
	_, err = th.ApplyTestMessage(st, th.VMStorage(), msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Contains(err.Error(), "second callSendTokens")
	assert.Contains(err.Error(), "not enough balance")

	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends and then spending directly
	params, err = abi.ToEncodedValues(addr1, addr2)
	assert.NoError(err)
	msg = types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "attemptMultiSpend2", params)
	_, err = th.ApplyTestMessage(st, th.VMStorage(), msg, types.NewBlockHeight(0))
	assert.Error(err)
	assert.Contains(err.Error(), "failed sendTokens")
	assert.Contains(err.Error(), "not enough balance")
}

func TestSendToNonexistentAddressThenSpendFromIt(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	cst := hamt.NewCborStore()

	mockSigner, _ := types.NewMockSignersAndKeyInfo(3)

	addr1, addr2, addr3 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2]
	addr4 := address.NewForTestGetter()()
	act1 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000))
	_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr1: act1,
	})

	// send 500 from addr1 to addr2
	msg := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(500), "", []byte{})
	smsg, err := types.NewSignedMessage(*msg, mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	_, err = NewDefaultProcessor().ApplyMessage(ctx, st, th.VMStorage(), smsg, addr4, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
	require.NoError(err)

	// send 250 along from addr2 to addr3
	msg = types.NewMessage(addr2, addr3, 0, types.NewAttoFILFromFIL(300), "", []byte{})
	smsg, err = types.NewSignedMessage(*msg, mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	_, err = NewDefaultProcessor().ApplyMessage(ctx, st, th.VMStorage(), smsg, addr4, types.NewBlockHeight(0), vm.NewGasTracker(), nil)
	require.NoError(err)

	// get all 3 actors
	act1 = state.MustGetActor(st, addr1)
	assert.Equal(types.NewAttoFILFromFIL(500), act1.Balance)
	assert.True(account.IsAccount(act1))

	act2 := state.MustGetActor(st, addr2)
	assert.Equal(types.NewAttoFILFromFIL(200), act2.Balance)
	assert.True(account.IsAccount(act2))

	act3 := state.MustGetActor(st, addr3)
	assert.Equal(types.NewAttoFILFromFIL(300), act3.Balance)
	assert.True(act3.Empty())
}

func TestApplyQueryMessageWillNotAlterState(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newAddress := address.NewForTestGetter()
	ctx := context.Background()
	cst := hamt.NewCborStore()
	vms := th.VMStorage()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
	defer func() {
		delete(builtin.Actors, fakeActorCodeCid)
	}()

	addr0, addr1, addr2 := newAddress(), newAddress(), newAddress()
	act0 := th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(101))
	act1 := th.RequireNewFakeActorWithTokens(require, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
	act2 := th.RequireNewFakeActorWithTokens(require, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
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

func TestApplyMessageChargesGas(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	vms := th.VMStorage()

	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
	defer delete(builtin.Actors, fakeActorCodeCid)

	t.Run("ApplyMessage charges gas on success", func(t *testing.T) {
		addresses, st, mockSigner := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
		addr0 := addresses[0]
		addr1 := addresses[1]
		minerAddr := addresses[2]

		msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "hasReturnValue", nil)
		gasPrice := types.NewAttoFILFromFIL(uint64(3))
		gasLimit := types.NewGasUnits(200)

		appResult, err := th.ApplyTestMessageWithGas(st, th.VMStorage(), msg, types.NewBlockHeight(0), mockSigner,
			*gasPrice, gasLimit, minerAddr)
		assert.NoError(err)
		assert.NoError(appResult.ExecutionError)

		minerActor, err := st.GetActor(ctx, minerAddr)
		require.NoError(err)
		// miner receives (3 FIL/gasUnit * 50 gasUnits) FIL from the sender
		assert.Equal(types.NewAttoFILFromFIL(1300), minerActor.Balance)
		accountActor, err := st.GetActor(ctx, addr0)
		require.NoError(err)
		// sender's resulting balance of FIL
		assert.Equal(types.NewAttoFILFromFIL(700), accountActor.Balance)
	})

	t.Run("ApplyMessage charges gas on message execution failure", func(t *testing.T) {
		addresses, st, mockSigner := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
		addr0 := addresses[0]
		addr1 := addresses[1]
		minerAddr := addresses[2]

		msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "chargeGasAndRevertError", nil)

		gasPrice := types.NewAttoFILFromFIL(uint64(3))
		gasLimit := types.NewGasUnits(200)

		appResult, err := th.ApplyTestMessageWithGas(st, th.VMStorage(), msg, types.NewBlockHeight(0), mockSigner,
			*gasPrice, gasLimit, minerAddr)
		assert.NoError(err)
		assert.EqualError(appResult.ExecutionError, "boom")

		minerActor, err := st.GetActor(ctx, minerAddr)
		require.NoError(err)

		// miner receives (3 FIL/gasUnit * 100 gasUnits) FIL from the sender
		assert.Equal(types.NewAttoFILFromFIL(1300), minerActor.Balance)
		accountActor, err := st.GetActor(ctx, addr0)
		require.NoError(err)
		assert.Equal(types.NewAttoFILFromFIL(700), accountActor.Balance)
	})

	t.Run("ApplyMessage charges the gas limit when limit is exceeded", func(t *testing.T) {
		// provide a gas limit less than the method charges.
		// call the method, expect an error and that gasLimit*gasPrice has been transferred to the miner.
		addresses, st, mockSigner := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
		addr0 := addresses[0]
		addr1 := addresses[1]
		minerAddr := addresses[2]
		msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "hasReturnValue", nil)

		gasPrice := types.NewAttoFILFromFIL(uint64(3))
		gasLimit := types.NewGasUnits(50)

		appResult, err := th.ApplyTestMessageWithGas(st, th.VMStorage(), msg, types.NewBlockHeight(0), mockSigner,
			*gasPrice, gasLimit, minerAddr)
		assert.NoError(err)
		assert.EqualError(appResult.ExecutionError, "Insufficient gas: gas cost exceeds gas limit")

		minerActor, err := st.GetActor(ctx, minerAddr)
		require.NoError(err)

		// miner receives (3 FIL/gasUnit * 100 gasUnits) FIL from the sender
		assert.Equal(types.NewAttoFILFromFIL(1150), minerActor.Balance)
		accountActor, err := st.GetActor(ctx, addr0)
		require.NoError(err)

		// sender's resulting balance of FIL
		assert.Equal(types.NewAttoFILFromFIL(850), accountActor.Balance)
	})

	t.Run("ApplyMessage when sending another message, with sufficient gas gets charged all the gas", func(t *testing.T) {
		addresses, st, mockSigner := setupActorsForGasTest(t, vms, fakeActorCodeCid, 2000)
		addr0 := addresses[0]
		addr1 := addresses[1]
		addr2 := addresses[2]
		minerAddr := addresses[3]

		params, err := abi.ToEncodedValues(addr2)
		assert.NoError(err)

		msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "runsAnotherMessage", params)

		gasPrice := types.NewAttoFILFromFIL(uint64(3))
		gasLimit := types.NewGasUnits(600)

		appResult, err := th.ApplyTestMessageWithGas(st, th.VMStorage(), msg, types.NewBlockHeight(0), mockSigner,
			*gasPrice, gasLimit, minerAddr)
		assert.NoError(err)
		assert.NoError(appResult.ExecutionError)
		minerActor, err := st.GetActor(ctx, minerAddr)
		require.NoError(err)

		// miner receives (3 FIL/gas * 100 gas * 2 messages)
		assert.Equal(types.NewAttoFILFromFIL(1600), minerActor.Balance)

		accountActor, err := st.GetActor(ctx, addr0)
		require.NoError(err)
		// sender's resulting balance of FIL
		assert.Equal(types.NewAttoFILFromFIL(1400), accountActor.Balance)
	})

	t.Run("ApplyMessage when it sends another message with insufficient gas fails with correct message", func(t *testing.T) {
		// provide a gas limit that is sufficient for the outer method's call, but insufficient for the inner
		// assert that it behaves as if the limit was exceeded after a single call.
		addresses, st, mockSigner := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
		addr0 := addresses[0]
		addr1 := addresses[1]
		addr2 := addresses[2]
		minerAddr := addresses[3]

		params, err := abi.ToEncodedValues(addr2)
		assert.NoError(err)

		msg := types.NewMessage(addr0, addr1, 0, types.ZeroAttoFIL, "runsAnotherMessage", params)

		gasPrice := types.NewAttoFILFromFIL(uint64(3))
		gasLimit := types.NewGasUnits(50)

		appResult, err := th.ApplyTestMessageWithGas(st, th.VMStorage(), msg, types.NewBlockHeight(0), mockSigner,
			*gasPrice, gasLimit, minerAddr)
		assert.NoError(err)
		assert.EqualError(appResult.ExecutionError, "Insufficient gas: gas cost exceeds gas limit")

		minerActor, err := st.GetActor(ctx, minerAddr)
		require.NoError(err)

		// miner receives (3 FIL/gasUnit * 100 gasUnits) FIL from the sender
		assert.Equal(types.NewAttoFILFromFIL(1150), minerActor.Balance)
		accountActor, err := st.GetActor(ctx, addr0)
		require.NoError(err)
		// sender's resulting balance of FIL
		assert.Equal(types.NewAttoFILFromFIL(850), accountActor.Balance)

	})
}

func TestBlockGasLimitBehavior(t *testing.T) {
	fakeActorCodeCid := types.NewCidForTestGetter()()
	builtin.Actors[fakeActorCodeCid] = &actor.FakeActor{}
	defer delete(builtin.Actors, fakeActorCodeCid)

	actors, stateTree, signer := setupActorsForGasTest(t, th.VMStorage(), fakeActorCodeCid, 0)
	sender := actors[1]
	receiver := actors[2]
	processor := NewTestProcessor()
	ctx := context.Background()

	t.Run("A single message whose gas limit is greater than the block gas limit fails permanently", func(t *testing.T) {
		msg := types.NewMessage(sender, receiver, 0, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg, err := types.NewSignedMessage(*msg, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*2)
		require.NoError(t, err)

		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.SignedMessage{sgnedMsg}, sender, types.NewBlockHeight(0), nil)
		require.NoError(t, err)

		assert.Contains(t, result.PermanentFailures, sgnedMsg)
	})

	t.Run("2 msgs both succeed when sum of limits > block limit, but 1st usage + 2nd limit < block limit", func(t *testing.T) {
		msg1 := types.NewMessage(sender, receiver, 0, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg1, err := types.NewSignedMessage(*msg1, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*5/8)
		require.NoError(t, err)

		msg2 := types.NewMessage(sender, receiver, 1, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg2, err := types.NewSignedMessage(*msg2, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*5/8)
		require.NoError(t, err)

		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.SignedMessage{sgnedMsg1, sgnedMsg2}, sender, types.NewBlockHeight(0), nil)
		require.NoError(t, err)

		assert.Contains(t, result.SuccessfulMessages, sgnedMsg1)
		assert.Contains(t, result.SuccessfulMessages, sgnedMsg2)
	})

	t.Run("2nd message delayed when 1st usage + 2nd limit > block limit", func(t *testing.T) {
		msg1 := types.NewMessage(sender, receiver, 0, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg1, err := types.NewSignedMessage(*msg1, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*3/8)
		require.NoError(t, err)

		msg2 := types.NewMessage(sender, receiver, 1, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg2, err := types.NewSignedMessage(*msg2, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*7/8)
		require.NoError(t, err)

		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.SignedMessage{sgnedMsg1, sgnedMsg2}, sender, types.NewBlockHeight(0), nil)
		require.NoError(t, err)

		assert.Contains(t, result.SuccessfulMessages, sgnedMsg1)
		assert.Contains(t, result.TemporaryFailures, sgnedMsg2)
	})

	t.Run("message with high gas limit does not block messages with lower limits from being included in block", func(t *testing.T) {
		msg1 := types.NewMessage(sender, receiver, 0, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg1, err := types.NewSignedMessage(*msg1, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*3/8)
		require.NoError(t, err)

		msg2 := types.NewMessage(sender, receiver, 1, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg2, err := types.NewSignedMessage(*msg2, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*7/8)
		require.NoError(t, err)

		msg3 := types.NewMessage(sender, receiver, 2, nil, "blockLimitTestMethod", []byte{})
		sgnedMsg3, err := types.NewSignedMessage(*msg3, signer, *types.NewZeroAttoFIL(), types.BlockGasLimit*3/8)
		require.NoError(t, err)

		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.SignedMessage{sgnedMsg1, sgnedMsg2, sgnedMsg3}, sender, types.NewBlockHeight(0), nil)
		require.NoError(t, err)

		assert.Contains(t, result.SuccessfulMessages, sgnedMsg1, sgnedMsg3)
		assert.Contains(t, result.TemporaryFailures, sgnedMsg2)
	})
}

func setupActorsForGasTest(t *testing.T, vms vm.StorageMap, fakeActorCodeCid cid.Cid, senderBalance uint64) ([]address.Address, state.Tree, *types.MockSigner) {
	require := require.New(t)

	addressGenerator := address.NewForTestGetter()

	mockSigner, _ := types.NewMockSignersAndKeyInfo(3)

	addresses := []address.Address{
		mockSigner.Addresses[0], // addr0
		mockSigner.Addresses[1], // addr1
		mockSigner.Addresses[2], // addr2
		addressGenerator()}      // minerAddr

	var actors []*actor.Actor
	// act0 sender account actor
	actors = append(actors, th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(senderBalance)))

	// act1 message recipient
	actors = append(actors, th.RequireNewFakeActorWithTokens(require, vms, addresses[1], fakeActorCodeCid, types.NewAttoFILFromFIL(100)))

	// act2 2nd message recipient
	actors = append(actors, th.RequireNewFakeActorWithTokens(require, vms, addresses[2], fakeActorCodeCid, types.NewAttoFILFromFIL(0)))

	// minerActor
	actors = append(actors, th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(0)))

	// networkActor
	actors = append(actors, th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(10000000)))

	cst := hamt.NewCborStore()
	cid, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addresses[0]:           actors[0],
		addresses[1]:           actors[1],
		addresses[2]:           actors[2],
		addresses[3]:           actors[3],
		address.NetworkAddress: actors[4],
	})
	require.NotNil(cid)

	return addresses, st, &mockSigner
}

func mustSetup2Actors(t *testing.T, balance1 *types.AttoFIL, balance2 *types.AttoFIL) (address.Address, *actor.Actor, address.Address, *actor.Actor, state.Tree, types.MockSigner) {
	require := require.New(t)

	cst := hamt.NewCborStore()
	vms := th.VMStorage()
	mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

	addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := th.RequireNewAccountActor(require, balance1)
	act2 := th.RequireNewMinerActor(require, vms, addr2, addr1, []byte{}, 10, th.RequireRandomPeerID(), balance2)

	_, st := requireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		addr1: act1,
		addr2: act2,
	})
	return addr1, act1, addr2, act2, st, mockSigner
}

func mustCreateMiner(ctx context.Context, require *require.Assertions, st state.Tree, vms vm.StorageMap, minerAddr, minerOwner address.Address) (cid.Cid, *actor.Actor) {
	miner := th.RequireNewMinerActor(require, vms, minerAddr, minerOwner, []byte{}, 1000, th.RequireRandomPeerID(), types.ZeroAttoFIL)
	st.SetActor(ctx, minerAddr, miner)
	stCid, err := st.Flush(ctx)
	require.NoError(err)
	return stCid, miner
}
