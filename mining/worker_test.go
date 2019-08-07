package mining_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func Test_Mine(t *testing.T) {
	tf.UnitTest(t)

	var doSomeWorkCalled bool
	CreatePoSTFunc := func() { doSomeWorkCalled = true }

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &types.Block{Height: 2, StateRoot: stateRoot}
	tipSet := th.RequireNewTipSet(t, baseBlock)

	st, pool, addrs, cst, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, ts types.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}

	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup

	messages := chain.NewMessageStore(cst)

	// TODO: this case isn't testing much.  Testing w.Mine further needs a lot more attention.
	t.Run("Trivial success case", func(t *testing.T) {
		doSomeWorkCalled = false
		ctx, cancel := context.WithCancel(context.Background())
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
			API: th.NewDefaultTestWorkerPorcelainAPI(),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			MinerWorker:    blockSignerAddr,
			WorkerSigner:   mockSigner,

			GetStateTree: getStateTree,
			GetWeight:    getWeightTest,
			GetAncestors: getAncestors,

			MessageSource: pool,
			Processor:     th.NewTestProcessor(),
			PowerTable:    mining.NewTestPowerTableView(1),
			Blockstore:    bs,
			MessageStore:  messages,
		}, CreatePoSTFunc)

		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		assert.NoError(t, r.Err)
		assert.True(t, doSomeWorkCalled)
		cancel()
	})
	t.Run("Block generation fails", func(t *testing.T) {
		doSomeWorkCalled = false
		ctx, cancel := context.WithCancel(context.Background())
		worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
			API: th.NewDefaultTestWorkerPorcelainAPI(),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			MinerWorker:    blockSignerAddr,
			WorkerSigner:   mockSigner,

			GetStateTree: makeExplodingGetStateTree(st),
			GetWeight:    getWeightTest,
			GetAncestors: getAncestors,

			MessageSource: pool,
			Processor:     th.NewTestProcessor(),
			PowerTable:    mining.NewTestPowerTableView(1),
			Blockstore:    bs,
			MessageStore:  messages,
		}, CreatePoSTFunc)
		outCh := make(chan mining.Output)
		doSomeWorkCalled = false
		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		assert.EqualError(t, r.Err, "generate flush state tree: boom no flush")
		assert.True(t, doSomeWorkCalled)
		cancel()

	})

	t.Run("Sent empty tipset", func(t *testing.T) {
		doSomeWorkCalled = false
		ctx, cancel := context.WithCancel(context.Background())
		worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
			API: th.NewDefaultTestWorkerPorcelainAPI(),

			MinerAddr:      minerAddr,
			MinerOwnerAddr: minerOwnerAddr,
			MinerWorker:    blockSignerAddr,
			WorkerSigner:   mockSigner,

			GetStateTree: getStateTree,
			GetWeight:    getWeightTest,
			GetAncestors: getAncestors,

			MessageSource: pool,
			Processor:     th.NewTestProcessor(),
			PowerTable:    mining.NewTestPowerTableView(1),
			Blockstore:    bs,
			MessageStore:  messages,
		}, CreatePoSTFunc)
		input := types.TipSet{}
		outCh := make(chan mining.Output)
		go worker.Mine(ctx, input, 0, outCh)
		r := <-outCh
		assert.EqualError(t, r.Err, "bad input tipset with no blocks sent to Mine()")
		assert.False(t, doSomeWorkCalled)
		cancel()
	})
}

func sharedSetupInitial() (*hamt.CborIpldStore, *core.MessagePool, cid.Cid) {
	cst := hamt.NewCborStore()
	pool := core.NewMessagePool(config.NewDefaultConfig().Mpool, th.NewMockMessagePoolValidator())
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.AccountActorCodeCid
	return cst, pool, fakeActorCodeCid
}

func sharedSetup(t *testing.T, mockSigner types.MockSigner) (
	state.Tree, *core.MessagePool, []address.Address, *hamt.CborIpldStore, blockstore.Blockstore) {

	cst, pool, fakeActorCodeCid := sharedSetupInitial()
	vms := th.VMStorage()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)

	// TODO: We don't need fake actors here, so these could be made real.
	//       And the NetworkAddress actor can/should be the real one.
	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2, addr3, addr4, addr5 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2], mockSigner.Addresses[3], mockSigner.Addresses[4]
	act1 := th.RequireNewFakeActor(t, vms, addr1, fakeActorCodeCid)
	act2 := th.RequireNewFakeActor(t, vms, addr2, fakeActorCodeCid)
	fakeNetAct := th.RequireNewFakeActorWithTokens(t, vms, addr3, fakeActorCodeCid, types.NewAttoFILFromFIL(1000000))
	minerAct := th.RequireNewMinerActor(t, vms, addr4, addr5, 10, th.RequireRandomPeerID(t), types.NewAttoFILFromFIL(10000))
	minerOwner := th.RequireNewFakeActor(t, vms, addr5, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
		// Ensure core.NetworkAddress exists to prevent mining reward failures.
		address.NetworkAddress: fakeNetAct,

		addr1: act1,
		addr2: act2,
		addr4: minerAct,
		addr5: minerOwner,
	})
	return st, pool, []address.Address{addr1, addr2, addr3, addr4, addr5}, cst, bs
}

// TODO this test belongs in core, it calls ApplyMessages
func TestApplyMessagesForSuccessTempAndPermFailures(t *testing.T) {
	tf.UnitTest(t)

	vms := th.VMStorage()

	mockSigner, _ := setupSigner()
	cst, _, fakeActorCodeCid := sharedSetupInitial()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := th.RequireNewFakeActor(t, vms, addr1, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(t, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(t, types.NewAttoFILFromFIL(1000000)),
		addr1:                  act1,
	})

	ctx := context.Background()

	// NOTE: it is important that each category (success, temporary failure, permanent failure) is represented below.
	// If a given message's category changes in the future, it needs to be replaced here in tests by another so we fully
	// exercise the categorization.
	// addr2 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addr2, addr1, 0, types.ZeroAttoFIL, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addr1, addr2, 0, types.ZeroAttoFIL, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addr1, addr1, 1, types.ZeroAttoFIL, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	msg4 := types.NewMessage(addr2, addr2, 1, types.ZeroAttoFIL, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	messages := []*types.SignedMessage{smsg1, smsg2, smsg3, smsg4}

	res, err := consensus.NewDefaultProcessor().ApplyMessagesAndPayRewards(ctx, st, vms, messages, addr1, types.NewBlockHeight(0), nil)

	assert.Len(t, res.PermanentFailures, 2)
	assert.Contains(t, res.PermanentFailures, smsg3)
	assert.Contains(t, res.PermanentFailures, smsg4)

	assert.Len(t, res.TemporaryFailures, 1)
	assert.Contains(t, res.TemporaryFailures, smsg1)

	assert.Len(t, res.Results, 1)
	assert.Contains(t, res.SuccessfulMessages, smsg2)

	assert.NoError(t, err)
}

func TestGenerateMultiBlockTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	CreatePoSTFunc := func() {}

	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs, cst, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts types.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}

	minerAddr := addrs[4]
	minerOwnerAddr := addrs[3]

	messages := chain.NewMessageStore(cst)

	worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
		API: th.NewDefaultTestWorkerPorcelainAPI(),

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		MinerWorker:    blockSignerAddr,
		WorkerSigner:   mockSigner,

		GetStateTree: getStateTree,
		GetWeight:    getWeightTest,
		GetAncestors: getAncestors,

		MessageSource: pool,
		Processor:     th.NewTestProcessor(),
		PowerTable:    &th.TestView{},
		Blockstore:    bs,
		MessageStore:  messages,
	}, CreatePoSTFunc)

	parents := types.NewTipSetKey(newCid())
	stateRoot := newCid()
	baseBlock1 := types.Block{
		Parents:      parents,
		Height:       types.Uint64(100),
		ParentWeight: types.Uint64(1000),
		StateRoot:    stateRoot,
	}
	baseBlock2 := types.Block{
		Parents:      parents,
		Height:       types.Uint64(100),
		ParentWeight: types.Uint64(1000),
		StateRoot:    stateRoot,
		Nonce:        1,
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock1, &baseBlock2), nil, types.PoStProof{}, 0)
	assert.NoError(t, err)

	assert.Equal(t, types.EmptyMessagesCID, blk.Messages)
	assert.Equal(t, types.EmptyReceiptsCID, blk.MessageReceipts)
	assert.Equal(t, types.Uint64(101), blk.Height)
	assert.Equal(t, types.Uint64(1020), blk.ParentWeight)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	tf.UnitTest(t)

	CreatePoSTFunc := func() {}

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs, cst, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, ts types.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}

	messages := chain.NewMessageStore(cst)

	worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
		API: th.NewDefaultTestWorkerPorcelainAPI(),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		MinerWorker:    blockSignerAddr,
		WorkerSigner:   mockSigner,

		GetStateTree: getStateTree,
		GetWeight:    getWeightTest,
		GetAncestors: getAncestors,

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		PowerTable:    &th.TestView{},
		Blockstore:    bs,
		MessageStore:  messages,
	}, CreatePoSTFunc)

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addrs[2], addrs[0], 0, types.ZeroAttoFIL, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	// add the following and then increment the actor nonce at addrs[1], nonceTooLow, a permanent error.
	msg3 := types.NewMessage(addrs[1], addrs[0], 0, types.ZeroAttoFIL, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	msg4 := types.NewMessage(addrs[1], addrs[2], 1, types.ZeroAttoFIL, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner, types.NewGasPrice(1), types.NewGasUnits(0))
	require.NoError(t, err)

	_, err = pool.Add(ctx, smsg1, 0)
	assert.NoError(t, err)
	_, err = pool.Add(ctx, smsg2, 0)
	assert.NoError(t, err)
	_, err = pool.Add(ctx, smsg3, 0)
	assert.NoError(t, err)
	_, err = pool.Add(ctx, smsg4, 0)
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 4)

	// Set actor nonce past nonce of message in pool.
	// Have to do this here to get a permanent error in the pool.
	act, err := st.GetActor(ctx, addrs[1])
	require.NoError(t, err)

	act.Nonce = types.Uint64(2)
	err = st.SetActor(ctx, addrs[1], act)
	require.NoError(t, err)

	stateRoot, err := st.Flush(ctx)
	require.NoError(t, err)

	baseBlock := types.Block{
		Parents:   types.NewTipSetKey(newCid()),
		Height:    types.Uint64(100),
		StateRoot: stateRoot,
		Proof:     types.PoStProof{},
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock), nil, types.PoStProof{}, 0)
	assert.NoError(t, err)

	// This is the temporary failure + the good message,
	// which will be removed by the node if this block is accepted.
	assert.Len(t, pool.Pending(), 2)
	assert.Contains(t, pool.Pending(), smsg1)
	assert.Contains(t, pool.Pending(), smsg2)

	// message and receipts can be loaded from message store and have
	// length 1.
	msgs, err := messages.LoadMessages(ctx, blk.Messages)
	require.NoError(t, err)
	assert.Len(t, msgs, 1) // This is the good message
	rcpts, err := messages.LoadReceipts(ctx, blk.MessageReceipts)
	require.NoError(t, err)
	assert.Len(t, rcpts, 1)
}

func TestGenerateSetsBasicFields(t *testing.T) {
	tf.UnitTest(t)

	CreatePoSTFunc := func() {}

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t, mockSigner)

	getStateTree := func(c context.Context, ts types.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}
	minerAddr := addrs[4]
	minerOwnerAddr := addrs[3]

	messages := chain.NewMessageStore(cst)

	worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
		API: th.NewDefaultTestWorkerPorcelainAPI(),

		MinerAddr:      minerAddr,
		MinerOwnerAddr: minerOwnerAddr,
		MinerWorker:    blockSignerAddr,
		WorkerSigner:   mockSigner,

		GetStateTree: getStateTree,
		GetWeight:    getWeightTest,
		GetAncestors: getAncestors,

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		PowerTable:    &th.TestView{},
		Blockstore:    bs,
		MessageStore:  messages,
	}, CreatePoSTFunc)

	h := types.Uint64(100)
	w := types.Uint64(1000)
	baseBlock := types.Block{
		Height:       h,
		ParentWeight: w,
		StateRoot:    newCid(),
		Proof:        types.PoStProof{},
	}
	baseTipSet := th.RequireNewTipSet(t, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, nil, types.PoStProof{}, 0)
	assert.NoError(t, err)

	assert.Equal(t, h+1, blk.Height)
	assert.Equal(t, minerAddr, blk.Miner)

	blk, err = worker.Generate(ctx, baseTipSet, nil, types.PoStProof{}, 1)
	assert.NoError(t, err)

	assert.Equal(t, h+2, blk.Height)
	assert.Equal(t, w+10.0, blk.ParentWeight)
	assert.Equal(t, minerAddr, blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	tf.UnitTest(t)

	CreatePoSTFunc := func() {}

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t, mockSigner)
	getStateTree := func(c context.Context, ts types.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}

	messages := chain.NewMessageStore(cst)

	worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
		API: th.NewDefaultTestWorkerPorcelainAPI(),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		MinerWorker:    blockSignerAddr,
		WorkerSigner:   mockSigner,

		GetStateTree: getStateTree,
		GetWeight:    getWeightTest,
		GetAncestors: getAncestors,

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		PowerTable:    &th.TestView{},
		Blockstore:    bs,
		MessageStore:  messages,
	}, CreatePoSTFunc)

	assert.Len(t, pool.Pending(), 0)
	baseBlock := types.Block{
		Parents:   types.NewTipSetKey(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
		Proof:     types.PoStProof{},
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(t, &baseBlock), nil, types.PoStProof{}, 0)
	assert.NoError(t, err)

	assert.Len(t, pool.Pending(), 0) // This is the temporary failure.

	assert.Equal(t, types.MessageCollection{}.Cid(), blk.Messages)
	assert.Equal(t, types.ReceiptCollection{}.Cid(), blk.MessageReceipts)
}

// If something goes wrong while generating a new block, even as late as when flushing it,
// no block should be returned, and the message pool should not be pruned.
func TestGenerateError(t *testing.T) {
	tf.UnitTest(t)

	CreatePoSTFunc := func() {}

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t, mockSigner)

	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}

	messages := chain.NewMessageStore(cst)
	worker := mining.NewDefaultWorkerWithDeps(mining.WorkerParameters{
		API: th.NewDefaultTestWorkerPorcelainAPI(),

		MinerAddr:      addrs[4],
		MinerOwnerAddr: addrs[3],
		MinerWorker:    blockSignerAddr,
		WorkerSigner:   mockSigner,

		GetStateTree: makeExplodingGetStateTree(st),
		GetWeight:    getWeightTest,
		GetAncestors: getAncestors,

		MessageSource: pool,
		Processor:     consensus.NewDefaultProcessor(),
		PowerTable:    &th.TestView{},
		Blockstore:    bs,
		MessageStore:  messages,
	}, CreatePoSTFunc)

	// This is actually okay and should result in a receipt
	msg := types.NewMessage(addrs[0], addrs[1], 0, types.ZeroAttoFIL, "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(t, err)
	_, err = pool.Add(ctx, smsg, 0)
	require.NoError(t, err)

	assert.Len(t, pool.Pending(), 1)
	baseBlock := types.Block{
		Parents:   types.NewTipSetKey(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
		Proof:     types.PoStProof{},
	}
	baseTipSet := th.RequireNewTipSet(t, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, nil, types.PoStProof{}, 0)
	assert.Error(t, err, "boom")
	assert.Nil(t, blk)

	assert.Len(t, pool.Pending(), 1) // No messages are removed from the pool.
}

type stateTreeForTest struct {
	state.Tree
	TestFlush func(ctx context.Context) (cid.Cid, error)
}

func wrapStateTreeForTest(st state.Tree) *stateTreeForTest {
	stt := stateTreeForTest{
		st,
		st.Flush,
	}
	return &stt
}

func (st *stateTreeForTest) Flush(ctx context.Context) (cid.Cid, error) {
	return st.TestFlush(ctx)
}

func getWeightTest(c context.Context, ts types.TipSet) (uint64, error) {
	w, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), err
	}
	return w + uint64(ts.Len())*consensus.ECV, nil
}

func makeExplodingGetStateTree(st state.Tree) func(context.Context, types.TipSet) (state.Tree, error) {
	return func(c context.Context, ts types.TipSet) (state.Tree, error) {
		stt := wrapStateTreeForTest(st)
		stt.TestFlush = func(ctx context.Context) (cid.Cid, error) {
			return cid.Undef, errors.New("boom no flush")
		}

		return stt, nil
	}
}

func setupSigner() (types.MockSigner, address.Address) {
	mockSigner, _ := types.NewMockSignersAndKeyInfo(10)

	signerAddr := mockSigner.Addresses[len(mockSigner.Addresses)-1]
	return mockSigner, signerAddr
}
