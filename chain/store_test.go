package chain_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// Note: many of these tests use the test chain defined in the init function of default_syncer_test.
func initStoreTest(ctx context.Context, t *testing.T, dstP *SyncerTestParams) {
	powerTable := &th.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	verifier := &verification.FakeVerifier{
		VerifyPoStValid: true,
	}
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), th.NewFakeBlockValidator(), powerTable, dstP.genCid, verifier, th.BlockTimeTest)
	initGenesisWrapper := func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
		return initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs)
	}
	initSyncTest(t, con, initGenesisWrapper, cst, bs, r, dstP, chain.Syncing)
	requireSetTestChain(t, con, true, dstP)
}

func newChainStore(dstP *SyncerTestParams) *chain.Store {
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	return chain.NewStore(ds, hamt.NewCborStore(), &state.TreeStateLoader{}, dstP.genCid)
}

// requirePutTestChain adds all test chain tipsets to the passed in chain store.
func requirePutTestChain(t *testing.T, chainStore *chain.Store, dstP *SyncerTestParams) {
	ctx := context.Background()
	genTsas := &chain.TipSetAndState{
		TipSet:          dstP.genTS,
		TipSetStateRoot: dstP.genStateRoot,
	}
	link1Tsas := &chain.TipSetAndState{
		TipSet:          dstP.link1,
		TipSetStateRoot: dstP.link1State,
	}
	link2Tsas := &chain.TipSetAndState{
		TipSet:          dstP.link2,
		TipSetStateRoot: dstP.link2State,
	}
	link3Tsas := &chain.TipSetAndState{
		TipSet:          dstP.link3,
		TipSetStateRoot: dstP.link3State,
	}

	link4Tsas := &chain.TipSetAndState{
		TipSet:          dstP.link4,
		TipSetStateRoot: dstP.link4State,
	}
	require.NoError(t, chainStore.PutTipSetAndState(ctx, genTsas))
	require.NoError(t, chainStore.PutTipSetAndState(ctx, link1Tsas))
	require.NoError(t, chainStore.PutTipSetAndState(ctx, link2Tsas))
	require.NoError(t, chainStore.PutTipSetAndState(ctx, link3Tsas))
	require.NoError(t, chainStore.PutTipSetAndState(ctx, link4Tsas))
}

func requireGetTsasByParentAndHeight(t *testing.T, chain *chain.Store, pKey string, h uint64) []*chain.TipSetAndState {
	tsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(pKey, h)
	require.NoError(t, err)
	return tsasSlice
}

type HeadAndTipsetGetter interface {
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

func requireHeadTipset(t *testing.T, chain HeadAndTipsetGetter) types.TipSet {
	headTipSet, err := chain.GetTipSet(chain.GetHead())
	require.NoError(t, err)
	return headTipSet
}

func requirePutBlocksToCborStore(t *testing.T, cst *hamt.CborIpldStore, blocks ...*types.Block) {
	for _, block := range blocks {
		_, err := cst.Put(context.Background(), block)
		require.NoError(t, err)
	}
}

/* Putting and getting tipsets and states. */

// Adding tipsets to the store doesn't error.
func TestPutTipSet(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	cs := newChainStore(dstP)
	genTsas := &chain.TipSetAndState{
		TipSet:          dstP.genTS,
		TipSetStateRoot: dstP.genStateRoot,
	}
	err := cs.PutTipSetAndState(ctx, genTsas)
	assert.NoError(t, err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)
	requirePutTestChain(t, chain, dstP)

	gotGTS := requireGetTipSet(ctx, t, chain, dstP.genTS.Key())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.genTS.Key())

	got1TS := requireGetTipSet(ctx, t, chain, dstP.link1.Key())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link1.Key())

	got2TS := requireGetTipSet(ctx, t, chain, dstP.link2.Key())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link2.Key())

	got3TS := requireGetTipSet(ctx, t, chain, dstP.link3.Key())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link3.Key())

	got4TS := requireGetTipSet(ctx, t, chain, dstP.link4.Key())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link4.Key())
	assert.Equal(t, dstP.genTS, gotGTS)
	assert.Equal(t, dstP.link1, got1TS)
	assert.Equal(t, dstP.link2, got2TS)
	assert.Equal(t, dstP.link3, got3TS)
	assert.Equal(t, dstP.link4, got4TS)

	assert.Equal(t, dstP.genStateRoot, gotGTSSR)
	assert.Equal(t, dstP.link1State, got1TSSR)
	assert.Equal(t, dstP.link2State, got2TSSR)
	assert.Equal(t, dstP.link3State, got3TSSR)
	assert.Equal(t, dstP.link4State, got4TSSR)
}

// Tipset state is loaded correctly
func TestGetTipSetState(t *testing.T) {
	ctx := context.Background()
	cst := hamt.NewCborStore()

	// setup testing state
	fakeCode := types.SomeCid()
	balance := types.NewAttoFILFromFIL(1000000)
	testActor := actor.NewActor(fakeCode, balance)
	addr := address.NewForTestGetter()()
	st1 := state.NewEmptyStateTree(cst)
	require.NoError(t, st1.SetActor(ctx, addr, testActor))
	root, err := st1.Flush(ctx)
	require.NoError(t, err)

	// link testing state to test block
	builder := chain.NewBuilder(t, address.Undef)
	gen := builder.NewGenesis()
	testTs := builder.BuildOn(gen, func(b *chain.BlockBuilder) {
		b.SetStateRoot(root)
	})

	// setup chain store
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	store := chain.NewStore(ds, cst, &state.TreeStateLoader{}, gen.At(0).Cid())

	// add tipset and state to chain store
	require.NoError(t, store.PutTipSetAndState(ctx, &chain.TipSetAndState{
		TipSet:          testTs,
		TipSetStateRoot: root,
	}))

	// verify output of GetTipSetState
	st2, err := store.GetTipSetState(ctx, testTs.Key())
	assert.NoError(t, err)
	for actRes := range state.GetAllActors(ctx, st2) {
		assert.NoError(t, actRes.Error)
		assert.Equal(t, addr.String(), actRes.Address)
		assert.Equal(t, fakeCode, actRes.Actor.Code)
		assert.Equal(t, testActor.Head, actRes.Actor.Head)
		assert.Equal(t, types.Uint64(0), actRes.Actor.Nonce)
		assert.Equal(t, balance, actRes.Actor.Balance)
	}
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)

	requirePutTestChain(t, chain, dstP)
	pkg := types.TipSetKey{}.String() // empty cid set is dstP.genesis pIDs
	pk1 := dstP.genTS.String()
	pk2 := dstP.link1.String()
	pk3 := dstP.link2.String()
	pk4 := dstP.link3.String()

	gotG := requireGetTsasByParentAndHeight(t, chain, pkg, uint64(0))
	got1 := requireGetTsasByParentAndHeight(t, chain, pk1, uint64(1))
	got2 := requireGetTsasByParentAndHeight(t, chain, pk2, uint64(2))
	got3 := requireGetTsasByParentAndHeight(t, chain, pk3, uint64(3))
	got4 := requireGetTsasByParentAndHeight(t, chain, pk4, uint64(6)) // two null blocks in between 3 and 4!

	assert.Equal(t, dstP.genTS, gotG[0].TipSet)
	assert.Equal(t, dstP.link1, got1[0].TipSet)
	assert.Equal(t, dstP.link2, got2[0].TipSet)
	assert.Equal(t, dstP.link3, got3[0].TipSet)
	assert.Equal(t, dstP.link4, got4[0].TipSet)

	assert.Equal(t, dstP.genStateRoot, gotG[0].TipSetStateRoot)
	assert.Equal(t, dstP.link1State, got1[0].TipSetStateRoot)
	assert.Equal(t, dstP.link2State, got2[0].TipSetStateRoot)
	assert.Equal(t, dstP.link3State, got3[0].TipSetStateRoot)
	assert.Equal(t, dstP.link4State, got4[0].TipSetStateRoot)
}

func TestGetMultipleByParent(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chainStore := newChainStore(dstP)

	mockSigner, ki := types.NewMockSignersAndKeyInfo(2)
	minerWorker, err := ki[0].Address()
	require.NoError(t, err)

	fakeChildParams := th.FakeChildParams{
		Parent:      dstP.genTS,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		MinerAddr:   dstP.minerAddress,
		Nonce:       uint64(5),
		Signer:      mockSigner,
		MinerWorker: minerWorker,
	}

	requirePutTestChain(t, chainStore, dstP)
	pk1 := dstP.genTS.String()
	// give one parent multiple children and then query
	newBlk := th.RequireMkFakeChild(t, fakeChildParams)
	newChild := th.RequireNewTipSet(t, newBlk)
	newRoot := dstP.cidGetter()
	newChildTsas := &chain.TipSetAndState{
		TipSet:          newChild,
		TipSetStateRoot: newRoot,
	}
	require.NoError(t, chainStore.PutTipSetAndState(ctx, newChildTsas))
	gotNew1 := requireGetTsasByParentAndHeight(t, chainStore, pk1, uint64(1))
	require.Equal(t, 2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if tsas.TipSet.Len() == 1 {
			assert.Equal(t, newRoot, tsas.TipSetStateRoot)
		} else {
			assert.Equal(t, dstP.link1State, tsas.TipSetStateRoot)
		}
	}
}

/* Head and its State is set and notified properly. */

// The constructor call sets the dstP.genesis block for the chain store.
func TestSetGenesis(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)
	requirePutTestChain(t, chain, dstP)
	require.Equal(t, dstP.genCid, chain.GenesisCid())
}

func assertSetHead(t *testing.T, chainStore *chain.Store, ts types.TipSet) {
	ctx := context.Background()
	err := chainStore.SetHead(ctx, ts)
	assert.NoError(t, err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)
	requirePutTestChain(t, chain, dstP)

	// Head starts as an empty cid set
	assert.Equal(t, types.TipSetKey{}, chain.GetHead())

	// Set Head
	assertSetHead(t, chain, dstP.genTS)
	assert.Equal(t, dstP.genTS.Key(), chain.GetHead())

	// Move head forward
	assertSetHead(t, chain, dstP.link4)
	assert.Equal(t, dstP.link4.Key(), chain.GetHead())

	// Move head back
	assertSetHead(t, chain, dstP.genTS)
	assert.Equal(t, dstP.genTS.Key(), chain.GetHead())
}

func assertEmptyCh(t *testing.T, ch <-chan interface{}) {
	select {
	case <-ch:
		assert.True(t, false)
	default:
	}
}

// Head events are propagated on HeadEvents.
func TestHeadEvents(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chainStore := newChainStore(dstP)
	requirePutTestChain(t, chainStore, dstP)

	ps := chainStore.HeadEvents()
	chA := ps.Sub(chain.NewHeadTopic)
	chB := ps.Sub(chain.NewHeadTopic)

	assertSetHead(t, chainStore, dstP.genTS)
	assertSetHead(t, chainStore, dstP.link1)
	assertSetHead(t, chainStore, dstP.link2)
	assertSetHead(t, chainStore, dstP.link3)
	assertSetHead(t, chainStore, dstP.link4)
	assertSetHead(t, chainStore, dstP.link3)
	assertSetHead(t, chainStore, dstP.link2)
	assertSetHead(t, chainStore, dstP.link1)
	assertSetHead(t, chainStore, dstP.genTS)
	heads := []types.TipSet{dstP.genTS, dstP.link1, dstP.link2, dstP.link3, dstP.link4, dstP.link3,
		dstP.link2, dstP.link1, dstP.genTS}

	// Heads arrive in the expected order
	for i := 0; i < 9; i++ {
		headA := <-chA
		headB := <-chB
		assert.Equal(t, headA, headB)
		assert.Equal(t, headA, heads[i])
	}

	// No extra notifications
	assertEmptyCh(t, chA)
	assertEmptyCh(t, chB)
}

/* Loading  */
// Load does not error and gives the chain store access to all blocks and
// tipset indexes along the heaviest chain.
func TestLoadAndReboot(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)

	rPriv := repo.NewInMemoryRepo()
	ds := rPriv.Datastore()
	cst := hamt.NewCborStore()
	// Add blocks to blockstore
	requirePutBlocksToCborStore(t, cst, dstP.genTS.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, dstP.link1.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, dstP.link2.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, dstP.link3.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, dstP.link4.ToSlice()...)

	chainStore := chain.NewStore(ds, cst, &state.TreeStateLoader{}, dstP.genCid)
	requirePutTestChain(t, chainStore, dstP)
	assertSetHead(t, chainStore, dstP.genTS) // set the genesis block

	assertSetHead(t, chainStore, dstP.link4)
	chainStore.Stop()

	// rebuild chain with same datastore and cborstore
	rebootChain := chain.NewStore(ds, cst, &state.TreeStateLoader{}, dstP.genCid)
	err := rebootChain.Load(ctx)
	assert.NoError(t, err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootChain, dstP.link2.Key())
	assert.Equal(t, dstP.link2, got2)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(t, rebootChain, dstP.link3.String(), uint64(6))
	assert.Equal(t, 1, len(got4))
	assert.Equal(t, dstP.link4, got4[0].TipSet)

	// Check the head
	assert.Equal(t, dstP.link4.Key(), rebootChain.GetHead())
}

type tipSetGetter interface {
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

func requireGetTipSet(ctx context.Context, t *testing.T, chainStore tipSetGetter, key types.TipSetKey) types.TipSet {
	ts, err := chainStore.GetTipSet(key)
	require.NoError(t, err)
	return ts
}

type tipSetStateRootGetter interface {
	GetTipSetStateRoot(tsKey types.TipSetKey) (cid.Cid, error)
}

func requireGetTipSetStateRoot(ctx context.Context, t *testing.T, chainStore tipSetStateRootGetter, key types.TipSetKey) cid.Cid {
	stateCid, err := chainStore.GetTipSetStateRoot(key)
	require.NoError(t, err)
	return stateCid
}
