package mining_test

import (
	"context"
	"errors"
	"testing"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func Test_Mine(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	var doSomeWorkCalled bool
	CreatePoSTFunc := func() { doSomeWorkCalled = true }

	mockSignerVal, blockSignerAddr := setupSigner()
	mockSigner := &mockSignerVal

	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &types.Block{Height: 2, StateRoot: stateRoot}
	tipSet := th.RequireNewTipSet(require, baseBlock)

	st, pool, addrs, cst, bs := sharedSetup(t, mockSignerVal)
	getStateTree := func(c context.Context, ts types.TipSet) (state.Tree, error) {
		return st, nil
	}
	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}

	minerAddr := addrs[3]      // addr4 in sharedSetup
	minerOwnerAddr := addrs[4] // addr5 in sharedSetup

	// TODO: this case isn't testing much.  Testing w.Mine further needs a lot more attention.
	t.Run("Trivial success case", func(t *testing.T) {
		doSomeWorkCalled = false
		ctx, cancel := context.WithCancel(context.Background())
		outCh := make(chan mining.Output)
		worker := mining.NewDefaultWorkerWithDeps(
			pool, getStateTree, getWeightTest, getAncestors, th.NewTestProcessor(), mining.NewTestPowerTableView(1),
			bs, cst, minerAddr, minerOwnerAddr, blockSignerAddr, mockSigner, th.BlockTimeTest,
			CreatePoSTFunc)

		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		assert.NoError(r.Err)
		assert.True(doSomeWorkCalled)
		cancel()
	})
	t.Run("Block generation fails", func(t *testing.T) {
		doSomeWorkCalled = false
		ctx, cancel := context.WithCancel(context.Background())
		worker := mining.NewDefaultWorkerWithDeps(pool, makeExplodingGetStateTree(st), getWeightTest, getAncestors, th.NewTestProcessor(),
			mining.NewTestPowerTableView(1), bs, cst, minerAddr, minerOwnerAddr, blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)
		outCh := make(chan mining.Output)
		doSomeWorkCalled = false
		go worker.Mine(ctx, tipSet, 0, outCh)
		r := <-outCh
		assert.EqualError(r.Err, "generate flush state tree: boom no flush")
		assert.True(doSomeWorkCalled)
		cancel()

	})

	t.Run("Sent empty tipset", func(t *testing.T) {
		doSomeWorkCalled = false
		ctx, cancel := context.WithCancel(context.Background())
		worker := mining.NewDefaultWorkerWithDeps(pool, getStateTree, getWeightTest, getAncestors, th.NewTestProcessor(),
			mining.NewTestPowerTableView(1), bs, cst, minerAddr, minerOwnerAddr, blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)
		input := types.TipSet{}
		outCh := make(chan mining.Output)
		go worker.Mine(ctx, input, 0, outCh)
		r := <-outCh
		assert.EqualError(r.Err, "bad input tipset with no blocks sent to Mine()")
		assert.False(doSomeWorkCalled)
		cancel()
	})
}

func sharedSetupInitial() (*hamt.CborIpldStore, *core.MessagePool, cid.Cid) {
	cst := hamt.NewCborStore()
	pool := core.NewMessagePool(th.NewTestBlockTimer(0))
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.AccountActorCodeCid
	return cst, pool, fakeActorCodeCid
}

func sharedSetup(t *testing.T, mockSigner types.MockSigner) (
	state.Tree, *core.MessagePool, []address.Address, *hamt.CborIpldStore, blockstore.Blockstore) {

	require := require.New(t)

	cst, pool, fakeActorCodeCid := sharedSetupInitial()
	vms := th.VMStorage()
	d := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(d)

	// TODO: We don't need fake actors here, so these could be made real.
	//       And the NetworkAddress actor can/should be the real one.
	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2, addr3, addr4, addr5 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2], mockSigner.Addresses[3], mockSigner.Addresses[4]
	act1 := th.RequireNewFakeActor(require, vms, addr1, fakeActorCodeCid)
	act2 := th.RequireNewFakeActor(require, vms, addr2, fakeActorCodeCid)
	fakeNetAct := th.RequireNewFakeActorWithTokens(require, vms, addr3, fakeActorCodeCid, types.NewAttoFILFromFIL(1000000))
	minerAct := th.RequireNewMinerActor(require, vms, addr4, addr5, []byte{}, 10, th.RequireRandomPeerID(), types.NewAttoFILFromFIL(10000))
	minerOwner := th.RequireNewFakeActor(require, vms, addr5, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
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
	assert := assert.New(t)
	require := require.New(t)
	vms := th.VMStorage()

	mockSigner, _ := setupSigner()
	cst, _, fakeActorCodeCid := sharedSetupInitial()

	// Stick two fake actors in the state tree so they can talk.
	addr1, addr2 := mockSigner.Addresses[0], mockSigner.Addresses[1]
	act1 := th.RequireNewFakeActor(require, vms, addr1, fakeActorCodeCid)
	_, st := th.RequireMakeStateTree(require, cst, map[address.Address]*actor.Actor{
		address.NetworkAddress: th.RequireNewAccountActor(require, types.NewAttoFILFromFIL(1000000)),
		addr1:                  act1,
	})

	ctx := context.Background()

	// NOTE: it is important that each category (success, temporary failure, permanent failure) is represented below.
	// If a given message's category changes in the future, it needs to be replaced here in tests by another so we fully
	// exercise the categorization.
	// addr2 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addr2, addr1, 0, nil, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addr1, addr2, 0, nil, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addr1, addr1, 1, nil, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	msg4 := types.NewMessage(addr2, addr2, 1, nil, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	messages := []*types.SignedMessage{smsg1, smsg2, smsg3, smsg4}

	res, err := consensus.NewDefaultProcessor().ApplyMessagesAndPayRewards(ctx, st, vms, messages, addr1, types.NewBlockHeight(0), nil)

	assert.Len(res.PermanentFailures, 2)
	assert.Contains(res.PermanentFailures, smsg3)
	assert.Contains(res.PermanentFailures, smsg4)

	assert.Len(res.TemporaryFailures, 1)
	assert.Contains(res.TemporaryFailures, smsg1)

	assert.Len(res.Results, 1)
	assert.Contains(res.SuccessfulMessages, smsg2)

	assert.NoError(err)
}

func TestGenerateMultiBlockTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
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

	worker := mining.NewDefaultWorkerWithDeps(pool, getStateTree, getWeightTest, getAncestors, th.NewTestProcessor(),
		&th.TestView{}, bs, cst, minerAddr, minerOwnerAddr, blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)

	parents := types.NewSortedCidSet(newCid())
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
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(require, &baseBlock1, &baseBlock2), nil, proofs.PoStProof{}, 0)
	assert.NoError(err)

	assert.Len(blk.Messages, 0)
	assert.Equal(types.Uint64(101), blk.Height)
	assert.Equal(types.Uint64(1020), blk.ParentWeight)
}

// After calling Generate, do the new block and new state of the message pool conform to our expectations?
func TestGeneratePoolBlockResults(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
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
	worker := mining.NewDefaultWorkerWithDeps(pool, getStateTree, getWeightTest, getAncestors, consensus.NewDefaultProcessor(),
		&th.TestView{}, bs, cst, addrs[4], addrs[3], blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addrs[2], addrs[0], 0, nil, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addrs[0], addrs[0], 1, nil, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	msg4 := types.NewMessage(addrs[1], addrs[1], 0, nil, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)

	pool.Add(smsg1)
	pool.Add(smsg2)
	pool.Add(smsg3)
	pool.Add(smsg4)

	assert.Len(pool.Pending(), 4)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
		Proof:     proofs.PoStProof{},
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(require, &baseBlock), nil, proofs.PoStProof{}, 0)
	assert.NoError(err)

	// This is the temporary failure + the good message,
	// which will be removed by the node if this block is accepted.
	assert.Len(pool.Pending(), 2)
	assert.Contains(pool.Pending(), smsg1)
	assert.Contains(pool.Pending(), smsg2)

	assert.Len(blk.Messages, 1) // This is the good message
}

func TestGenerateSetsBasicFields(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

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
	worker := mining.NewDefaultWorkerWithDeps(pool, getStateTree, getWeightTest, getAncestors, consensus.NewDefaultProcessor(),
		&th.TestView{}, bs, cst, minerAddr, minerOwnerAddr, blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)

	h := types.Uint64(100)
	w := types.Uint64(1000)
	baseBlock := types.Block{
		Height:       h,
		ParentWeight: w,
		StateRoot:    newCid(),
		Proof:        proofs.PoStProof{},
	}
	baseTipSet := th.RequireNewTipSet(require, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, nil, proofs.PoStProof{}, 0)
	assert.NoError(err)

	assert.Equal(h+1, blk.Height)
	assert.Equal(minerAddr, blk.Miner)

	blk, err = worker.Generate(ctx, baseTipSet, nil, proofs.PoStProof{}, 1)
	assert.NoError(err)

	assert.Equal(h+2, blk.Height)
	assert.Equal(w+10.0, blk.ParentWeight)
	assert.Equal(minerAddr, blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

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
	worker := mining.NewDefaultWorkerWithDeps(pool, getStateTree, getWeightTest, getAncestors, consensus.NewDefaultProcessor(),
		&th.TestView{}, bs, cst, addrs[4], addrs[3], blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)

	assert.Len(pool.Pending(), 0)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
		Proof:     proofs.PoStProof{},
	}
	blk, err := worker.Generate(ctx, th.RequireNewTipSet(require, &baseBlock), nil, proofs.PoStProof{}, 0)
	assert.NoError(err)

	assert.Len(pool.Pending(), 0) // This is the temporary failure.
	assert.Len(blk.Messages, 0)
}

// If something goes wrong while generating a new block, even as late as when flushing it,
// no block should be returned, and the message pool should not be pruned.
func TestGenerateError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	CreatePoSTFunc := func() {}

	ctx := context.Background()
	mockSigner, blockSignerAddr := setupSigner()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t, mockSigner)

	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return nil, nil
	}
	worker := mining.NewDefaultWorkerWithDeps(pool, makeExplodingGetStateTree(st), getWeightTest, getAncestors,
		consensus.NewDefaultProcessor(),
		&th.TestView{}, bs, cst, addrs[4], addrs[3], blockSignerAddr, mockSigner, th.BlockTimeTest, CreatePoSTFunc)

	// This is actually okay and should result in a receipt
	msg := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner, types.NewGasPrice(0), types.NewGasUnits(0))
	require.NoError(err)
	pool.Add(smsg)

	assert.Len(pool.Pending(), 1)
	baseBlock := types.Block{
		Parents:   types.NewSortedCidSet(newCid()),
		Height:    types.Uint64(100),
		StateRoot: newCid(),
		Proof:     proofs.PoStProof{},
	}
	baseTipSet := th.RequireNewTipSet(require, &baseBlock)
	blk, err := worker.Generate(ctx, baseTipSet, nil, proofs.PoStProof{}, 0)
	assert.Error(err, "boom")
	assert.Nil(blk)

	assert.Len(pool.Pending(), 1) // No messages are removed from the pool.
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
	return w + uint64(len(ts))*consensus.ECV, nil
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

func setupSigner() (types.MockSigner, []byte) {
	mockSigner, _ := types.NewMockSignersAndKeyInfo(10)

	signerPubKey := mockSigner.PubKeys[len(mockSigner.Addresses)-1]
	return mockSigner, signerPubKey
}
