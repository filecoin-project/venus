package mining

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-filecoin/proofs"
	"testing"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
)

func Test_Mine(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	newCid := types.NewCidForTestGetter()
	stateRoot := newCid()
	baseBlock := &types.Block{Height: 2, StateRoot: stateRoot}
	tipSet := th.RequireNewTipSet(require, baseBlock)
	ctx, cancel := context.WithCancel(context.Background())

	st, pool, addrs, cst, bs := sharedSetup(t)
	getStateTree := func(c context.Context, ts consensus.TipSet) (state.Tree, error) {
		return st, nil
	}

	// Success case. TODO: this case isn't testing much.  Testing w.Mine
	// further needs a lot more attention.
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, th.NewTestProcessor(), NewTestPowerTableView(1), bs, cst, addrs[3], th.BlockTimeTest)

	outCh := make(chan Output)
	doSomeWorkCalled := false
	worker.createPoST = func() { doSomeWorkCalled = true }
	go worker.Mine(ctx, tipSet, 0, outCh)
	r := <-outCh
	assert.NoError(r.Err)
	assert.True(doSomeWorkCalled)
	cancel()
	// Block generation fails.
	ctx, cancel = context.WithCancel(context.Background())
	worker = NewDefaultWorker(pool, makeExplodingGetStateTree(st), getWeightTest, th.NewTestProcessor(), NewTestPowerTableView(1), bs, cst, addrs[3], th.BlockTimeTest)
	outCh = make(chan Output)
	doSomeWorkCalled = false
	worker.createPoST = func() { doSomeWorkCalled = true }
	go worker.Mine(ctx, tipSet, 0, outCh)
	r = <-outCh
	assert.Error(r.Err)
	assert.True(doSomeWorkCalled)
	cancel()

	// Sent empty tipset
	ctx, cancel = context.WithCancel(context.Background())
	worker = NewDefaultWorker(pool, getStateTree, getWeightTest, th.NewTestProcessor(), NewTestPowerTableView(1), bs, cst, addrs[3], th.BlockTimeTest)
	outCh = make(chan Output)
	doSomeWorkCalled = false
	worker.createPoST = func() { doSomeWorkCalled = true }
	input := consensus.TipSet{}
	go worker.Mine(ctx, input, 0, outCh)
	r = <-outCh
	assert.Error(r.Err)
	assert.False(doSomeWorkCalled)
	cancel()
}

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)

func TestGenerate(t *testing.T) {
	// TODO use core.FakeActor for state/contract tests for generate:
	//  - test nonces out of order
	//  - test nonce gap
}

func sharedSetupInitial() (*hamt.CborIpldStore, *core.MessagePool, cid.Cid) {
	cst := hamt.NewCborStore()
	pool := core.NewMessagePool()
	// Install the fake actor so we can execute it.
	fakeActorCodeCid := types.AccountActorCodeCid
	return cst, pool, fakeActorCodeCid
}

func sharedSetup(t *testing.T) (state.Tree, *core.MessagePool, []address.Address, *hamt.CborIpldStore, blockstore.Blockstore) {
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
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addr1, addr2, 0, nil, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addr1, addr1, 1, nil, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	msg4 := types.NewMessage(addr2, addr2, 1, nil, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	messages := []*types.SignedMessage{smsg1, smsg2, smsg3, smsg4}

	res, err := consensus.NewDefaultProcessor().ApplyMessagesAndPayRewards(ctx, st, vms, messages, addr1, types.NewBlockHeight(0))

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
	newCid := types.NewCidForTestGetter()
	st, pool, addrs, cst, bs := sharedSetup(t)
	getStateTree := func(c context.Context, ts consensus.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, th.NewTestProcessor(), &th.TestView{}, bs, cst, addrs[3], th.BlockTimeTest)

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

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()
	st, pool, addrs, cst, bs := sharedSetup(t)

	getStateTree := func(c context.Context, ts consensus.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, consensus.NewDefaultProcessor(), &th.TestView{}, bs, cst, addrs[3], th.BlockTimeTest)

	// addr3 doesn't correspond to an extant account, so this will trigger errAccountNotFound -- a temporary failure.
	msg1 := types.NewMessage(addrs[2], addrs[0], 0, nil, "", nil)
	smsg1, err := types.NewSignedMessage(*msg1, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	// This is actually okay and should result in a receipt
	msg2 := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	smsg2, err := types.NewSignedMessage(*msg2, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	// The following two are sending to self -- errSelfSend, a permanent error.
	msg3 := types.NewMessage(addrs[0], addrs[0], 1, nil, "", nil)
	smsg3, err := types.NewSignedMessage(*msg3, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
	require.NoError(err)

	msg4 := types.NewMessage(addrs[1], addrs[1], 0, nil, "", nil)
	smsg4, err := types.NewSignedMessage(*msg4, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
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

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t)

	getStateTree := func(c context.Context, ts consensus.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, consensus.NewDefaultProcessor(), &th.TestView{}, bs, cst, addrs[3], th.BlockTimeTest)

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
	assert.Equal(addrs[3], blk.Miner)

	blk, err = worker.Generate(ctx, baseTipSet, nil, proofs.PoStProof{}, 1)
	assert.NoError(err)

	assert.Equal(h+2, blk.Height)
	assert.Equal(w+10.0, blk.ParentWeight)
	assert.Equal(addrs[3], blk.Miner)
}

func TestGenerateWithoutMessages(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t)
	getStateTree := func(c context.Context, ts consensus.TipSet) (state.Tree, error) {
		return st, nil
	}
	worker := NewDefaultWorker(pool, getStateTree, getWeightTest, consensus.NewDefaultProcessor(), &th.TestView{}, bs, cst, addrs[3], th.BlockTimeTest)

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
	ctx := context.Background()
	newCid := types.NewCidForTestGetter()

	st, pool, addrs, cst, bs := sharedSetup(t)

	worker := NewDefaultWorker(pool, makeExplodingGetStateTree(st), getWeightTest, consensus.NewDefaultProcessor(), &th.TestView{}, bs, cst, addrs[3], th.BlockTimeTest)

	// This is actually okay and should result in a receipt
	msg := types.NewMessage(addrs[0], addrs[1], 0, nil, "", nil)
	smsg, err := types.NewSignedMessage(*msg, &mockSigner, types.NewGasPrice(0), types.NewGasCost(0))
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

type StateTreeForTest struct {
	state.Tree
	TestFlush func(ctx context.Context) (cid.Cid, error)
}

func WrapStateTreeForTest(st state.Tree) *StateTreeForTest {
	stt := StateTreeForTest{
		st,
		st.Flush,
	}
	return &stt
}

func (st *StateTreeForTest) Flush(ctx context.Context) (cid.Cid, error) {
	return st.TestFlush(ctx)
}

func getWeightTest(c context.Context, ts consensus.TipSet) (uint64, error) {
	w, err := ts.ParentWeight()
	if err != nil {
		return uint64(0), err
	}
	return w + uint64(len(ts))*consensus.ECV, nil
}

func makeExplodingGetStateTree(st state.Tree) func(context.Context, consensus.TipSet) (state.Tree, error) {
	return func(c context.Context, ts consensus.TipSet) (state.Tree, error) {
		stt := WrapStateTreeForTest(st)
		stt.TestFlush = func(ctx context.Context) (cid.Cid, error) {
			return cid.Undef, errors.New("boom no flush")
		}

		return stt, nil
	}
}
