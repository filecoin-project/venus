package consensus_test

// import (
// 	"context"
// 	"testing"

// 	"github.com/ipfs/go-cid"
// 	"github.com/ipfs/go-datastore"
// 	"github.com/ipfs/go-hamt-ipld"
// 	blockstore "github.com/ipfs/go-ipfs-blockstore"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"

// 	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
// 	. "github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
// 	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
// 	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
// 	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
// )

// func requireMakeStateTree(t *testing.T, cst *hamt.CborIpldStore, acts map[address.Address]*actor.Actor) (cid.Cid, state.Tree) {
// 	ctx := context.Background()
// 	tree := state.NewTree(cst)

// 	for addr, act := range acts {
// 		err := tree.SetActor(ctx, addr, act)
// 		require.NoError(t, err)
// 	}

// 	c, err := tree.Flush(ctx)
// 	require.NoError(t, err)

// 	return c, tree
// }

// func TestProcessTipSetSuccess(t *testing.T) {
// 	tf.UnitTest(t)

// 	newAddress := address.NewForTestGetter()
// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()

// 	startingNetworkBalance := types.NewAttoFILFromFIL(1000000)

// 	toAddr := newAddress()
// 	mockSigner, _ := types.NewMockSignersAndKeyInfo(3)

// 	fromAddr1 := mockSigner.Addresses[0]
// 	fromAddr2 := mockSigner.Addresses[1]
// 	minerOwner := mockSigner.Addresses[2]

// 	vms := th.VMStorage()
// 	_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		address.LegacyNetworkAddress: th.RequireNewAccountActor(t, startingNetworkBalance),
// 	})
// 	th.RequireInitAccountActor(ctx, t, st, vms, fromAddr1, types.NewAttoFILFromFIL(10000))
// 	th.RequireInitAccountActor(ctx, t, st, vms, fromAddr2, types.NewAttoFILFromFIL(10000))

// 	stCid, _, minerAddr := mustCreateStorageMiner(ctx, t, st, vms, minerOwner)

// 	msg1 := types.NewMeteredMessage(fromAddr1, toAddr, 0, types.NewAttoFILFromFIL(550), types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(300))
// 	msgs1 := []*types.UnsignedMessage{msg1}
// 	cidGetter := types.NewCidForTestGetter()
// 	blk1 := &block.Block{
// 		Height:    20,
// 		StateRoot: stCid,
// 		Miner:     minerAddr,
// 		Messages:  types.TxMeta{SecpRoot: cidGetter(), BLSRoot: types.EmptyMessagesCID},
// 		Ticket:    block.Ticket{VRFProof: []byte{0x1}},
// 	}

// 	msgs2 := []*types.UnsignedMessage{types.NewMeteredMessage(fromAddr2, toAddr, 0,
// 		types.NewAttoFILFromFIL(50), types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(300))}
// 	blk2 := &block.Block{
// 		Height:    20,
// 		StateRoot: stCid,
// 		Miner:     minerAddr,
// 		Messages:  types.TxMeta{SecpRoot: cidGetter(), BLSRoot: types.EmptyMessagesCID},
// 		Ticket:    block.Ticket{VRFProof: []byte{0x2}},
// 	}

// 	tsMsgs := [][]*types.UnsignedMessage{msgs1, msgs2}
// 	res, err := NewDefaultProcessor().ProcessTipSet(ctx, st, vms, th.RequireNewTipSet(t, blk1, blk2), tsMsgs, nil)
// 	require.NoError(t, err)
// 	assert.Len(t, res, 2)
// 	for _, r := range res {
// 		require.NoError(t, r.Failure)
// 		assert.Equal(t, uint8(0), r.Receipt.ExitCode)
// 	}

// 	_, err = st.Flush(ctx)
// 	require.NoError(t, err)
// }

// func TestProcessTipsConflicts(t *testing.T) {
// 	tf.UnitTest(t)

// 	startingNetworkBalance := types.NewAttoFILFromFIL(1000000)

// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()
// 	mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

// 	fromAddr, toAddr := mockSigner.Addresses[0], mockSigner.Addresses[1]
// 	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		address.LegacyNetworkAddress: th.RequireNewAccountActor(t, startingNetworkBalance),
// 	})
// 	th.RequireInitAccountActor(ctx, t, st, vms, fromAddr, types.NewAttoFILFromFIL(1000))

// 	minerOwner, err := address.NewSecp256k1Address([]byte("mo"))
// 	require.NoError(t, err)
// 	stCid, _, minerAddr := mustCreateStorageMiner(ctx, t, st, vms, minerOwner)

// 	msgs1 := []*types.UnsignedMessage{types.NewMeteredMessage(fromAddr, toAddr, 0,
// 		types.NewAttoFILFromFIL(501), types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))}
// 	blk1 := &block.Block{
// 		Height:    20,
// 		StateRoot: stCid,
// 		Ticket:    block.Ticket{VRFProof: []byte{0, 0}}, // Block with smaller ticket
// 		Miner:     minerAddr,
// 	}

// 	msgs2 := []*types.UnsignedMessage{types.NewMeteredMessage(fromAddr, toAddr, 0,
// 		types.NewAttoFILFromFIL(502), types.SendMethodID, nil, types.NewGasPrice(1), types.NewGasUnits(0))}
// 	blk2 := &block.Block{
// 		Height:    20,
// 		StateRoot: stCid,
// 		Ticket:    block.Ticket{VRFProof: []byte{1, 1}},
// 		Miner:     minerAddr,
// 	}

// 	tsMsgs := [][]*types.UnsignedMessage{msgs1, msgs2}
// 	res, err := NewDefaultProcessor().ProcessTipSet(ctx, st, vms, th.RequireNewTipSet(t, blk1, blk2), tsMsgs, nil)
// 	require.NoError(t, err)
// 	assert.Len(t, res, 2)
// 	require.NoError(t, res[0].Failure)
// 	assert.Equal(t, uint8(0), res[0].Receipt.ExitCode)
// 	assert.Error(t, res[1].Failure)
// 	// Insufficient balance to cover gas is marked as a permanent error, but probably shouldn't be.
// 	assert.True(t, res[1].FailureIsPermanent)

// 	_, err = st.Flush(ctx)
// 	require.NoError(t, err)
// }

// // ProcessTipset should not fail with an unsigned block reward message.
// func TestProcessTipsetReward(t *testing.T) {
// 	tf.UnitTest(t)

// 	newAddress := address.NewForTestGetter()
// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()

// 	minerOwnerAddr := newAddress()
// 	minerBalance := types.NewAttoFILFromFIL(10000)
// 	networkAct := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(100000000000))
// 	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		address.LegacyNetworkAddress: networkAct,
// 	})
// 	_, ownerIDAddr := th.RequireInitAccountActor(ctx, t, st, vms, minerOwnerAddr, minerBalance)

// 	stCid, _, minerAddr := mustCreateStorageMiner(ctx, t, st, vms, minerOwnerAddr)

// 	blk := &block.Block{
// 		Miner:     minerAddr,
// 		Height:    20,
// 		StateRoot: stCid,
// 	}
// 	results, err := NewDefaultProcessor().ProcessTipSet(ctx, st, vms, RequireNewTipSet(require.New(t), blk), [][]*types.UnsignedMessage{{}}, nil)
// 	require.NoError(t, err)
// 	assert.Len(t, results, 0)

// 	minerOwnerActor, err := st.GetActor(ctx, ownerIDAddr)
// 	require.NoError(t, err)

// 	blockRewardAmount := NewDefaultBlockRewarder().BlockRewardAmount()
// 	assert.Equal(t, minerBalance.Add(blockRewardAmount), minerOwnerActor.Balance)
// }

// func TestProcessTipsetVMErrors(t *testing.T) {
// 	tf.BadUnitTestWithSideEffects(t)

// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()

// 	newAddress := address.NewForTestGetter()
// 	startingNetworkBalance := types.NewAttoFILFromFIL(1000000)
// 	minerAddr, minerOwnerAddr := newAddress(), newAddress()

// 	// Install the fake actor so we can execute it.
// 	fakeActorCodeCid := types.NewCidForTestGetter()()
// 	actors := builtin.NewBuilder().
// 		AddAll(builtin.DefaultActors).
// 		Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
// 		Build()
// 	mockSigner, _ := types.NewMockSignersAndKeyInfo(2)

// 	// Stick one empty actor and one fake actor in the state tree so they can talk.
// 	fromAddr := mockSigner.Addresses[0]
// 	toAddr, err := address.NewIDAddress(42)
// 	require.NoError(t, err)

// 	act2 := th.RequireNewFakeActor(t, vms, toAddr, fakeActorCodeCid)
// 	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		address.LegacyNetworkAddress: th.RequireNewAccountActor(t, startingNetworkBalance),
// 		address.InitAddress:          th.RequireNewInitActor(t, vms),
// 		toAddr:                       act2,
// 	})
// 	_, fromID := th.RequireInitAccountActor(ctx, t, st, vms, fromAddr, types.NewAttoFILFromFIL(1000))
// 	th.RequireInitAccountActor(ctx, t, st, vms, minerOwnerAddr, types.ZeroAttoFIL)

// 	stCid, _, minerAddr := mustCreateStorageMiner(ctx, t, st, vms, minerOwnerAddr)

// 	msgs := [][]*types.UnsignedMessage{{types.NewMeteredMessage(fromAddr, toAddr, 0, types.NewAttoFILFromFIL(1000),
// 		actor.ReturnRevertErrorID, nil, types.NewGasPrice(1), types.NewGasUnits(0))}}
// 	blk := &block.Block{
// 		Height:    20,
// 		StateRoot: stCid,
// 		Miner:     minerAddr,
// 	}

// 	// The "foo" message will cause a vm error and
// 	// we're going to check four things...
// 	processor := NewConfiguredProcessor(NewDefaultMessageValidator(), NewDefaultBlockRewarder(), actors)
// 	results, err := processor.ProcessTipSet(ctx, st, vms, RequireNewTipSet(require.New(t), blk), msgs, nil)

// 	// 1. That a VM error is not a message failure (err).
// 	require.NoError(t, err)

// 	// 2. That the VM error is faithfully recorded.
// 	require.Len(t, results, 1)
// 	assert.Len(t, results[0].Receipt.Return, 0)
// 	assert.Contains(t, results[0].ExecutionError.Error(), "boom")

// 	// 3 & 4. That on VM error the state is rolled back and nonce is inc'd.
// 	fromAct, err := st.GetActor(ctx, fromID)
// 	require.NoError(t, err)
// 	assert.Equal(t, types.Uint64(1), fromAct.CallSeqNum)
// 	assert.Equal(t, types.NewAttoFILFromFIL(1000), fromAct.Balance) // state rollback leaves balance intact
// }

// func TestProcessTipsetParamsLengthError(t *testing.T) {
// 	tf.UnitTest(t)

// 	newAddress := address.NewForTestGetter()
// 	ctx := context.TODO()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()
// 	_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})

// 	addr1 := newAddress()
// 	th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(1000))
// 	_, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, uint64(10), th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))

// 	params, err := abi.ToValues([]interface{}{"param"})
// 	require.NoError(t, err)
// 	badParams, err := abi.EncodeValues(params)
// 	require.NoError(t, err)
// 	msg := types.NewUnsignedMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), miner.GetPower, badParams)

// 	rct, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
// 	require.NoError(t, err) // No error means definitely no fault error, which is what we're especially testing here.

// 	assert.Empty(t, rct.Receipt.Return)
// 	assert.Contains(t, rct.ExecutionError.Error(), "invalid params: expected 0 parameters, but got 1")
// }

// func TestProcessTipsetParamsError(t *testing.T) {
// 	tf.UnitTest(t)

// 	newAddress := address.NewForTestGetter()
// 	ctx := context.TODO()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()
// 	_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})

// 	addr1 := newAddress()
// 	th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(1000))
// 	_, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, uint64(10), th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
// 	badParams := []byte{1, 2, 3, 4, 5}
// 	msg := types.NewUnsignedMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), miner.GetPower, badParams)

// 	rct, err := th.ApplyTestMessage(st, vms, msg, types.NewBlockHeight(0))
// 	require.NoError(t, err) // No error means definitely no fault error, which is what we're especially testing here.

// 	assert.Empty(t, rct.Receipt.Return)
// 	assert.Contains(t, rct.ExecutionError.Error(), "invalid params: malformed stream")
// }

// func TestApplyMessagesValidation(t *testing.T) {
// 	tf.UnitTest(t)

// 	t.Run("Errors when nonce too high", func(t *testing.T) {
// 		ctx := context.TODO()
// 		cst := hamt.NewCborStore()
// 		vms := th.VMStorage()
// 		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

// 		addr1 := mockSigner.Addresses[0]
// 		_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})
// 		th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(1000))
// 		_, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
// 		msg := types.NewMeteredMessage(addr1, addr2, 5, types.NewAttoFILFromFIL(550), types.SendMethodID, []byte{}, types.NewGasPrice(1), types.NewGasUnits(0))

// 		_, err := NewDefaultProcessor().ApplyMessage(ctx, st, vms, msg, addr2, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		assert.Error(t, err)
// 		assert.Equal(t, "nonce too high", err.(*errors.ApplyErrorTemporary).Cause().Error())
// 	})

// 	t.Run("Errors when nonce too low", func(t *testing.T) {
// 		ctx := context.Background()
// 		cst := hamt.NewCborStore()
// 		vms := th.VMStorage()
// 		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

// 		addr1 := mockSigner.Addresses[0]
// 		_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})
// 		act1, idAddr := th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(1000))
// 		act1.CallSeqNum = 5
// 		require.NoError(t, st.SetActor(ctx, idAddr, act1))

// 		_, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, uint64(10), th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
// 		msg := types.NewMeteredMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), types.SendMethodID, []byte{}, types.NewGasPrice(1), types.NewGasUnits(0))

// 		_, err := NewDefaultProcessor().ApplyMessage(ctx, st, vms, msg, addr2, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		assert.Error(t, err)
// 		assert.Equal(t, "nonce too low", err.(*errors.ApplyErrorPermanent).Cause().Error())
// 	})

// 	t.Run("errors when specifying a gas limit in excess of balance", func(t *testing.T) {
// 		addr1, _, addr2, _, st, vms, _ := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
// 		msg := types.NewMeteredMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), types.SendMethodID, []byte{}, types.NewAttoFILFromFIL(10), types.NewGasUnits(50))

// 		// the maximum gas charge (10*50 = 500) is greater than the sender balance minus the message value (1000-550 = 450)
// 		_, err := NewDefaultProcessor().ApplyMessage(context.Background(), st, vms, msg, addr2, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		require.Error(t, err)
// 		assert.Equal(t, "balance insufficient to cover transfer+gas", err.(*errors.ApplyErrorPermanent).Cause().Error())
// 	})

// 	t.Run("errors when sender is not an account actor", func(t *testing.T) {
// 		_, _, addr2, _, st, vms, _ := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
// 		addr1, err := address.NewIDAddress(42)
// 		require.NoError(t, err)
// 		act1 := th.RequireNewFakeActor(t, vms, addr1, types.NewCidForTestGetter()())

// 		ctx := context.Background()
// 		require.NoError(t, st.SetActor(ctx, addr1, act1))

// 		msg := types.NewMeteredMessage(addr1, addr2, 0, types.ZeroAttoFIL, types.SendMethodID, []byte{}, types.NewAttoFILFromFIL(10), types.NewGasUnits(50))

// 		_, err = NewDefaultProcessor().ApplyMessage(context.Background(), st, vms, msg, addr2, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		require.Error(t, err)
// 		assert.Equal(t, "message from non-account actor", err.(*errors.ApplyErrorPermanent).Cause().Error())
// 	})

// 	t.Run("errors when sender is not an actor", func(t *testing.T) {
// 		ctx := context.TODO()
// 		cst := hamt.NewCborStore()
// 		vms := th.VMStorage()
// 		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

// 		addr1 := mockSigner.Addresses[0]

// 		_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})
// 		_, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(1000))

// 		msg := types.NewMeteredMessage(addr1, addr2, 0, types.ZeroAttoFIL, types.SendMethodID, []byte{}, types.NewAttoFILFromFIL(10), types.NewGasUnits(50))

// 		_, err := NewDefaultProcessor().ApplyMessage(context.Background(), st, vms, msg, addr2,
// 			types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		require.Error(t, err)
// 		assert.Equal(t, "from (sender) account not found", err.(*errors.ApplyErrorTemporary).Cause().Error())
// 	})

// 	t.Run("errors on attempt to transfer negative value", func(t *testing.T) {
// 		ctx := context.Background()
// 		cst := hamt.NewCborStore()
// 		vms := th.VMStorage()
// 		mockSigner, _ := types.NewMockSignersAndKeyInfo(1)
// 		_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})

// 		addr1 := mockSigner.Addresses[0]
// 		th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(1000))

// 		_, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))

// 		someval, ok := types.NewAttoFILFromString("-500", 10)
// 		require.True(t, ok)

// 		msg := types.NewMeteredMessage(addr1, addr2, 0, someval, types.SendMethodID, []byte{}, types.NewGasPrice(1), types.NewGasUnits(0))

// 		_, err := NewDefaultProcessor().ApplyMessage(ctx, st, vms, msg, addr2, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		assert.Error(t, err)
// 		assert.Contains(t, "negative value", err.(*errors.ApplyErrorPermanent).Cause().Error())
// 	})

// 	t.Run("errors when attempting to send to self", func(t *testing.T) {
// 		addr1, _, addr2, _, st, vms, _ := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
// 		msg := types.NewMeteredMessage(addr1, addr1, 0, types.NewAttoFILFromFIL(550), types.SendMethodID, []byte{}, types.NewAttoFILFromFIL(10), types.NewGasUnits(0))

// 		// the maximum gas charge (10*50 = 500) is greater than the sender balance minus the message value (1000-550 = 450)
// 		_, err := NewDefaultProcessor().ApplyMessage(context.Background(), st, vms, msg, addr2, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		require.Error(t, err)
// 		assert.Equal(t, "cannot send to self", err.(*errors.ApplyErrorPermanent).Cause().Error())
// 	})

// 	t.Run("errors when specifying a gas limit in excess of balance", func(t *testing.T) {
// 		addr1, _, addr2, _, st, vms, _ := mustSetup2Actors(t, types.NewAttoFILFromFIL(1000), types.NewAttoFILFromFIL(10000))
// 		msg := types.NewMeteredMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(550), types.SendMethodID, []byte{}, types.NewAttoFILFromFIL(10), types.NewGasUnits(50))

// 		// the maximum gas charge (10*50 = 500) is greater than the sender balance minus the message value (1000-550 = 450)
// 		_, err := NewDefaultProcessor().ApplyMessage(context.Background(), st, vms, msg, address.Undef, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 		require.Error(t, err)
// 		assert.Equal(t, "balance insufficient to cover transfer+gas", err.(*errors.ApplyErrorPermanent).Cause().Error())
// 	})
// }

// // TODO add more test cases that cover the intent expressed
// // in ApplyMessage's comments.

// func TestNestedSendBalance(t *testing.T) {
// 	tf.UnitTest(t)

// 	newAddress := address.NewForTestGetter()
// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()
// 	bs := blockstore.NewBlockstore(datastore.NewMapDatastore())
// 	vms := vm.NewStorageMap(bs)

// 	// Install the fake actor so we can execute it.
// 	fakeActorCodeCid := types.NewCidForTestGetter()()
// 	actors := builtin.NewBuilder().
// 		AddAll(builtin.DefaultActors).
// 		Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
// 		Build()

// 	addr1, err := address.NewIDAddress(110)
// 	require.NoError(t, err)
// 	addr2, err := address.NewIDAddress(111)
// 	require.NoError(t, err)
// 	act1 := th.RequireNewFakeActorWithTokens(t, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
// 	act2 := th.RequireNewFakeActorWithTokens(t, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

// 	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		addr1: act1,
// 		addr2: act2,
// 	})
// 	addr0 := newAddress()
// 	_, act0ID := th.RequireInitAccountActor(ctx, t, st, vms, addr0, types.NewAttoFILFromFIL(101))

// 	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
// 	params1, err := abi.ToEncodedValues(addr2)
// 	require.NoError(t, err)
// 	msg1 := types.NewUnsignedMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.NestedBalanceID, params1)

// 	_, err = th.ApplyTestMessageWithActors(actors, st, vms, msg1, types.NewBlockHeight(0))
// 	require.NoError(t, err)

// 	_, err = st.Flush(ctx)
// 	require.NoError(t, err)

// 	expAct0, err := st.GetActor(ctx, act0ID)
// 	require.NoError(t, err)
// 	assert.Equal(t, types.NewAttoFILFromFIL(101), expAct0.Balance)
// 	assert.Equal(t, types.Uint64(1), expAct0.CallSeqNum)

// 	expAct1, err := st.GetActor(ctx, addr1)
// 	require.NoError(t, err)
// 	assert.Equal(t, types.NewAttoFILFromFIL(2), expAct1.Balance)

// 	expAct2, err := st.GetActor(ctx, addr2)
// 	require.NoError(t, err)
// 	assert.Equal(t, types.NewAttoFILFromFIL(100), expAct2.Balance)
// }

// func TestReentrantTransferDoesntAllowMultiSpending(t *testing.T) {
// 	tf.BadUnitTestWithSideEffects(t)

// 	newAddress := address.NewForTestGetter()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()
// 	ctx := context.TODO()

// 	// Install the fake actor so we can execute it.
// 	fakeActorCodeCid := types.NewCidForTestGetter()()
// 	actors := builtin.NewBuilder().
// 		AddAll(builtin.DefaultActors).
// 		Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
// 		Build()

// 	addr1, err := address.NewIDAddress(110)
// 	require.NoError(t, err)
// 	addr2, err := address.NewIDAddress(111)
// 	require.NoError(t, err)
// 	act1 := th.RequireNewFakeActorWithTokens(t, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(100))
// 	act2 := th.RequireNewFakeActorWithTokens(t, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

// 	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		addr1: act1,
// 		addr2: act2,
// 	})
// 	addr0 := newAddress()
// 	th.RequireInitAccountActor(ctx, t, st, vms, addr0, types.NewAttoFILFromFIL(0))

// 	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends twice
// 	params, err := abi.ToEncodedValues(addr1, addr2)
// 	require.NoError(t, err)
// 	msg := types.NewUnsignedMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.AttemptMultiSpend1ID, params)
// 	_, err = th.ApplyTestMessageWithActors(actors, st, vms, msg, types.NewBlockHeight(0))
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "second callSendTokens")
// 	assert.Contains(t, err.Error(), "not enough balance")

// 	// addr1 will attempt to double spend to addr2 by sending a reentrant message that spends and then spending directly
// 	params, err = abi.ToEncodedValues(addr1, addr2)
// 	require.NoError(t, err)
// 	msg = types.NewUnsignedMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.AttemptMultiSpend2ID, params)
// 	_, err = th.ApplyTestMessageWithActors(actors, st, vms, msg, types.NewBlockHeight(0))
// 	require.Error(t, err)
// 	assert.Contains(t, err.Error(), "failed sendTokens")
// 	assert.Contains(t, err.Error(), "not enough balance")
// }

// func TestSendToNonexistentAddressThenSpendFromIt(t *testing.T) {
// 	tf.UnitTest(t)

// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()

// 	mockSigner, _ := types.NewMockSignersAndKeyInfo(3)
// 	_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})
// 	vms := th.VMStorage()

// 	addr1, addr2, addr3 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2]
// 	addr4 := address.NewForTestGetter()()
// 	_, addr1ID := th.RequireInitAccountActor(ctx, t, st, vms, addr1, types.NewAttoFILFromFIL(1000))

// 	// send 500 from addr1 to addr2
// 	msg := types.NewMeteredMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(500), types.SendMethodID, []byte{}, types.NewGasPrice(1), types.NewGasUnits(0))
// 	_, err := NewDefaultProcessor().ApplyMessage(ctx, st, vms, msg, addr4, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 	require.NoError(t, err)

// 	initActor, _ := st.GetActor(ctx, address.InitAddress)
// 	s := vms.NewStorage(address.InitAddress, initActor)

// 	_, err = s.Get(initActor.Head)
// 	require.NoError(t, err)

// 	// send 250 along from addr2 to addr3
// 	msg = types.NewMeteredMessage(addr2, addr3, 0, types.NewAttoFILFromFIL(300), types.SendMethodID, []byte{}, types.NewGasPrice(1), types.NewGasUnits(0))
// 	_, err = NewDefaultProcessor().ApplyMessage(ctx, st, vms, msg, addr4, types.NewBlockHeight(0), vm.NewLegacyGasTracker(), nil)
// 	require.NoError(t, err)

// 	initActor, _ = st.GetActor(ctx, address.InitAddress)
// 	s = vms.NewStorage(address.InitAddress, initActor)

// 	_, err = s.Get(initActor.Head)
// 	require.NoError(t, err)

// 	// get all 3 actors
// 	act1 := state.MustGetActor(st, addr1ID)
// 	assert.Equal(t, types.NewAttoFILFromFIL(500), act1.Balance)
// 	assert.True(t, types.AccountActorCodeCid.Equals(act1.Code))

// 	act2, _ := th.RequireLookupActor(ctx, t, st, vms, addr2)
// 	assert.Equal(t, types.NewAttoFILFromFIL(200), act2.Balance)
// 	assert.True(t, types.AccountActorCodeCid.Equals(act2.Code))

// 	act3, _ := th.RequireLookupActor(ctx, t, st, vms, addr3)
// 	assert.Equal(t, types.NewAttoFILFromFIL(300), act3.Balance)
// }

// func TestApplyQueryMessageWillNotAlterState(t *testing.T) {
// 	tf.BadUnitTestWithSideEffects(t)

// 	newAddress := address.NewForTestGetter()
// 	ctx := context.Background()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()

// 	// Install the fake actor so we can execute it.
// 	fakeActorCodeCid := types.NewCidForTestGetter()()
// 	actors := builtin.NewBuilder().
// 		AddAll(builtin.DefaultActors).
// 		Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
// 		Build()

// 	addr1, err := address.NewIDAddress(110)
// 	require.NoError(t, err)
// 	addr2, err := address.NewIDAddress(111)
// 	require.NoError(t, err)
// 	act1 := th.RequireNewFakeActorWithTokens(t, vms, addr1, fakeActorCodeCid, types.NewAttoFILFromFIL(102))
// 	act2 := th.RequireNewFakeActorWithTokens(t, vms, addr2, fakeActorCodeCid, types.NewAttoFILFromFIL(0))

// 	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		addr1: act1,
// 		addr2: act2,
// 	})
// 	addr0 := newAddress()
// 	th.RequireInitAccountActor(ctx, t, st, vms, addr0, types.NewAttoFILFromFIL(101))

// 	// pre-execution state
// 	preCid, err := st.Flush(ctx)
// 	require.NoError(t, err)

// 	// send 100 from addr1 -> addr2, by sending a message from addr0 to addr1
// 	args1, err := abi.ToEncodedValues(addr2)
// 	require.NoError(t, err)

// 	processor := NewConfiguredProcessor(NewDefaultMessageValidator(), NewDefaultBlockRewarder(), actors)
// 	_, exitCode, err := processor.CallQueryMethod(ctx, st, vms, addr1, actor.NestedBalanceID, args1, addr0, types.NewBlockHeight(0))
// 	require.Equal(t, uint8(0), exitCode)
// 	require.NoError(t, err)

// 	// post-execution state
// 	postCid, err := st.Flush(ctx)
// 	require.NoError(t, err)
// 	assert.True(t, preCid.Equals(postCid))
// }

// func TestApplyMessageChargesGas(t *testing.T) {
// 	tf.BadUnitTestWithSideEffects(t)

// 	ctx := context.Background()

// 	// Install the fake actor so we can execute it.
// 	fakeActorCodeCid := types.NewCidForTestGetter()()
// 	actors := builtin.NewBuilder().
// 		AddAll(builtin.DefaultActors).
// 		Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
// 		Build()

// 	t.Run("ApplyMessage charges gas on success", func(t *testing.T) {
// 		vms := th.VMStorage()
// 		addresses, st := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
// 		addr0 := addresses[0]
// 		addr1 := addresses[1]
// 		minerAddr := addresses[2]

// 		gasPrice := types.NewAttoFILFromFIL(uint64(3))
// 		gasLimit := types.NewGasUnits(200)
// 		msg := types.NewMeteredMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.HasReturnValueID, nil, gasPrice, gasLimit)

// 		appResult, err := th.ApplyTestMessageWithGas(actors, st, vms, msg, types.NewBlockHeight(0), minerAddr)
// 		require.NoError(t, err)
// 		require.NoError(t, appResult.ExecutionError)

// 		minerActor, err := st.GetActor(ctx, minerAddr)
// 		require.NoError(t, err)
// 		// miner receives (3 FIL/gasUnit * 50 gasUnits) FIL from the sender
// 		assert.Equal(t, types.NewAttoFILFromFIL(1300), minerActor.Balance)
// 		accountActor, _ := th.RequireLookupActor(ctx, t, st, vms, addr0)
// 		// sender's resulting balance of FIL
// 		assert.Equal(t, types.NewAttoFILFromFIL(700), accountActor.Balance)
// 	})

// 	t.Run("ApplyMessage charges gas on message execution failure", func(t *testing.T) {
// 		vms := th.VMStorage()
// 		addresses, st := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
// 		addr0 := addresses[0]
// 		addr1 := addresses[1]
// 		minerAddr := addresses[2]

// 		gasPrice := types.NewAttoFILFromFIL(uint64(3))
// 		gasLimit := types.NewGasUnits(200)
// 		msg := types.NewMeteredMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.ChargeGasAndRevertErrorID, nil, gasPrice, gasLimit)

// 		appResult, err := th.ApplyTestMessageWithGas(actors, st, vms, msg, types.NewBlockHeight(0), minerAddr)
// 		require.NoError(t, err)
// 		assert.EqualError(t, appResult.ExecutionError, "boom")

// 		minerActor, err := st.GetActor(ctx, minerAddr)
// 		require.NoError(t, err)

// 		// miner receives (3 FIL/gasUnit * 100 gasUnits) FIL from the sender
// 		assert.Equal(t, types.NewAttoFILFromFIL(1300), minerActor.Balance)
// 		accountActor, _ := th.RequireLookupActor(ctx, t, st, vms, addr0)
// 		assert.Equal(t, types.NewAttoFILFromFIL(700), accountActor.Balance)
// 	})

// 	t.Run("ApplyMessage charges the gas limit when limit is exceeded", func(t *testing.T) {
// 		vms := th.VMStorage()
// 		// provide a gas limit less than the method charges.
// 		// call the method, expect an error and that gasLimit*gasPrice has been transferred to the miner.
// 		addresses, st := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
// 		addr0 := addresses[0]
// 		addr1 := addresses[1]
// 		minerAddr := addresses[2]

// 		gasPrice := types.NewAttoFILFromFIL(uint64(3))
// 		gasLimit := types.NewGasUnits(50)
// 		msg := types.NewMeteredMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.HasReturnValueID, nil, gasPrice, gasLimit)

// 		appResult, err := th.ApplyTestMessageWithGas(actors, st, vms, msg, types.NewBlockHeight(0), minerAddr)
// 		require.NoError(t, err)
// 		assert.EqualError(t, appResult.ExecutionError, "Insufficient gas: gas cost exceeds gas limit")

// 		minerActor, err := st.GetActor(ctx, minerAddr)
// 		require.NoError(t, err)

// 		// miner receives (3 FIL/gasUnit * 100 gasUnits) FIL from the sender
// 		assert.Equal(t, types.NewAttoFILFromFIL(1150), minerActor.Balance)
// 		accountActor, _ := th.RequireLookupActor(ctx, t, st, vms, addr0)

// 		// sender's resulting balance of FIL
// 		assert.Equal(t, types.NewAttoFILFromFIL(850), accountActor.Balance)
// 	})

// 	t.Run("ApplyMessage when sending another message, with sufficient gas gets charged all the gas", func(t *testing.T) {
// 		vms := th.VMStorage()
// 		addresses, st := setupActorsForGasTest(t, vms, fakeActorCodeCid, 2000)
// 		addr0 := addresses[0]
// 		addr1 := addresses[1]
// 		addr2 := addresses[2]
// 		minerAddr := addresses[3]

// 		params, err := abi.ToEncodedValues(addr2)
// 		require.NoError(t, err)

// 		gasPrice := types.NewAttoFILFromFIL(uint64(3))
// 		gasLimit := types.NewGasUnits(600)
// 		msg := types.NewMeteredMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.RunsAnotherMessageID, params, gasPrice, gasLimit)

// 		appResult, err := th.ApplyTestMessageWithGas(actors, st, vms, msg, types.NewBlockHeight(0), minerAddr)
// 		require.NoError(t, err)
// 		require.NoError(t, appResult.ExecutionError)
// 		minerActor, err := st.GetActor(ctx, minerAddr)
// 		require.NoError(t, err)

// 		// miner receives (3 FIL/gas * 100 gas * 2 messages)
// 		assert.Equal(t, types.NewAttoFILFromFIL(1600), minerActor.Balance)

// 		accountActor, _ := th.RequireLookupActor(ctx, t, st, vms, addr0)
// 		// sender's resulting balance of FIL
// 		assert.Equal(t, types.NewAttoFILFromFIL(1400), accountActor.Balance)
// 	})

// 	t.Run("ApplyMessage when it sends another message with insufficient gas fails with correct message", func(t *testing.T) {
// 		vms := th.VMStorage()
// 		// provide a gas limit that is sufficient for the outer method's call, but insufficient for the inner
// 		// assert that it behaves as if the limit was exceeded after a single call.
// 		addresses, st := setupActorsForGasTest(t, vms, fakeActorCodeCid, 1000)
// 		addr0 := addresses[0]
// 		addr1 := addresses[1]
// 		addr2 := addresses[2]
// 		minerAddr := addresses[3]

// 		params, err := abi.ToEncodedValues(addr2)
// 		require.NoError(t, err)

// 		gasPrice := types.NewAttoFILFromFIL(uint64(3))
// 		gasLimit := types.NewGasUnits(50)
// 		msg := types.NewMeteredMessage(addr0, addr1, 0, types.ZeroAttoFIL, actor.RunsAnotherMessageID, params, gasPrice, gasLimit)

// 		appResult, err := th.ApplyTestMessageWithGas(actors, st, vms, msg, types.NewBlockHeight(0), minerAddr)
// 		require.NoError(t, err)
// 		assert.EqualError(t, appResult.ExecutionError, "Insufficient gas: gas cost exceeds gas limit")

// 		minerActor, err := st.GetActor(ctx, minerAddr)
// 		require.NoError(t, err)

// 		// miner receives (3 FIL/gasUnit * 100 gasUnits) FIL from the sender
// 		assert.Equal(t, types.NewAttoFILFromFIL(1150), minerActor.Balance)
// 		accountActor, _ := th.RequireLookupActor(ctx, t, st, vms, addr0)
// 		// sender's resulting balance of FIL
// 		assert.Equal(t, types.NewAttoFILFromFIL(850), accountActor.Balance)

// 	})
// }

// func TestBlockGasLimitBehavior(t *testing.T) {
// 	tf.BadUnitTestWithSideEffects(t)

// 	fakeActorCodeCid := types.NewCidForTestGetter()()
// 	builtinActors := builtin.NewBuilder().
// 		AddAll(builtin.DefaultActors).
// 		Add(fakeActorCodeCid, 0, &actor.FakeActor{}).
// 		Build()

// 	actors, stateTree := setupActorsForGasTest(t, th.VMStorage(), fakeActorCodeCid, 0)
// 	sender := actors[1]
// 	receiver := actors[2]
// 	processor := NewFakeProcessor(builtinActors)
// 	ctx := context.Background()

// 	t.Run("A single message whose gas limit is greater than the block gas limit fails permanently", func(t *testing.T) {
// 		msg := types.NewMeteredMessage(sender, receiver, 0, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*2)

// 		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.UnsignedMessage{msg}, sender, types.NewBlockHeight(0), nil)
// 		require.NoError(t, err)

// 		assert.Error(t, result[0].Failure)
// 		assert.True(t, result[0].FailureIsPermanent)
// 	})

// 	t.Run("2 msgs both succeed when sum of limits > block limit, but 1st usage + 2nd limit < block limit", func(t *testing.T) {
// 		msg1 := types.NewMeteredMessage(sender, receiver, 0, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*5/8)
// 		msg2 := types.NewMeteredMessage(sender, receiver, 1, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*5/8)

// 		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.UnsignedMessage{msg1, msg2}, sender, types.NewBlockHeight(0), nil)
// 		require.NoError(t, err)

// 		assert.Nil(t, result[0].Failure)
// 		assert.Nil(t, result[1].Failure)
// 	})

// 	t.Run("2nd message delayed when 1st usage + 2nd limit > block limit", func(t *testing.T) {
// 		msg1 := types.NewMeteredMessage(sender, receiver, 0, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*3/8)
// 		msg2 := types.NewMeteredMessage(sender, receiver, 1, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*7/8)

// 		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.UnsignedMessage{msg1, msg2}, sender, types.NewBlockHeight(0), nil)
// 		require.NoError(t, err)

// 		assert.Nil(t, result[0].Failure)
// 		assert.Error(t, result[1].Failure)
// 		assert.False(t, result[0].FailureIsPermanent)
// 	})

// 	t.Run("message with high gas limit does not block messages with lower limits from being included in block", func(t *testing.T) {
// 		msg1 := types.NewMeteredMessage(sender, receiver, 0, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*3/8)
// 		msg2 := types.NewMeteredMessage(sender, receiver, 1, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*7/8)
// 		msg3 := types.NewMeteredMessage(sender, receiver, 2, types.ZeroAttoFIL, actor.BlockLimitTestMethodID, []byte{}, types.ZeroAttoFIL, types.BlockGasLimit*3/8)

// 		result, err := processor.ApplyMessagesAndPayRewards(ctx, stateTree, th.VMStorage(), []*types.UnsignedMessage{msg1, msg2, msg3}, sender, types.NewBlockHeight(0), nil)
// 		require.NoError(t, err)

// 		assert.Nil(t, result[0].Failure)
// 		assert.Error(t, result[1].Failure)
// 		assert.False(t, result[0].FailureIsPermanent)
// 		assert.Nil(t, result[2].Failure)
// 	})
// }

// func setupActorsForGasTest(t *testing.T, vms vm.StorageMap, fakeActorCodeCid cid.Cid, senderBalance uint64) ([]address.Address, state.Tree) {
// 	ctx := context.TODO()

// 	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

// 	addr1, err := address.NewIDAddress(110)
// 	require.NoError(t, err)
// 	addr2, err := address.NewIDAddress(111)
// 	require.NoError(t, err)
// 	addr3, err := address.NewIDAddress(112)
// 	require.NoError(t, err)

// 	addresses := []address.Address{
// 		mockSigner.Addresses[0], // addr0
// 		addr1,                   // addr1
// 		addr2,                   // addr2
// 		addr3,                   // minerAddr
// 	}

// 	// act1 message recipient
// 	act1 := th.RequireNewFakeActorWithTokens(t, vms, addresses[1], fakeActorCodeCid, types.NewAttoFILFromFIL(100))

// 	// act2 2nd message recipient
// 	act2 := th.RequireNewFakeActorWithTokens(t, vms, addresses[2], fakeActorCodeCid, types.NewAttoFILFromFIL(0))

// 	// minerActor
// 	act3 := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(0))

// 	// networkActor
// 	act4 := th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(10000000))

// 	cst := hamt.NewCborStore()
// 	cid, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
// 		addresses[1]:                 act1,
// 		addresses[2]:                 act2,
// 		addresses[3]:                 act3,
// 		address.LegacyNetworkAddress: act4,
// 	})
// 	require.NotNil(t, cid)

// 	// sender account actor
// 	th.RequireInitAccountActor(ctx, t, st, vms, addresses[0], types.NewAttoFILFromFIL(senderBalance))

// 	return addresses, st
// }

// func mustSetup2Actors(t *testing.T, balance1 types.AttoFIL, balance2 types.AttoFIL) (address.Address, *actor.Actor, address.Address, *actor.Actor, state.Tree, vm.StorageMap, types.MockSigner) {
// 	ctx := context.TODO()
// 	cst := hamt.NewCborStore()
// 	vms := th.VMStorage()
// 	mockSigner, _ := types.NewMockSignersAndKeyInfo(1)

// 	_, st := requireMakeStateTree(t, cst, map[address.Address]*actor.Actor{})

// 	addr1 := mockSigner.Addresses[0]
// 	act1, _ := th.RequireInitAccountActor(ctx, t, st, vms, addr1, balance1)

// 	act2, addr2 := th.RequireNewMinerActor(ctx, t, st, vms, addr1, 10, th.RequireRandomPeerID(t), balance2)
// 	return addr1, act1, addr2, act2, st, vms, mockSigner
// }

// func mustCreateStorageMiner(ctx context.Context, t *testing.T, st state.Tree, vms vm.StorageMap, minerOwner address.Address) (cid.Cid, *actor.Actor, address.Address) {
// 	miner, minerAddr := th.RequireNewMinerActor(ctx, t, st, vms, minerOwner, 1000, th.RequireRandomPeerID(t), types.ZeroAttoFIL)
// 	stCid, err := st.Flush(ctx)
// 	require.NoError(t, err)
// 	return stCid, miner, minerAddr
// }
