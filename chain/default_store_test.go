package chain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

// Note: many of these tests use the test chain defined in the init function of default_syncer_test.
func initStoreTest(ctx context.Context, t *testing.T) {
	powerTable := &th.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), powerTable, genCid, proofs.NewFakeVerifier(true, nil))
	initSyncTest(t, con, initGenesis, cst, bs, r)
	requireSetTestChain(t, con, true)
}

func newChainStore() chain.Store {
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	return chain.NewDefaultStore(ds, genCid)
}

// requirePutTestChain adds all test chain tipsets to the passed in chain store.
func requirePutTestChain(t *testing.T, chainStore chain.Store) {
	ctx := context.Background()
	genTsas := &chain.TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genStateRoot,
	}
	link1Tsas := &chain.TipSetAndState{
		TipSet:          link1,
		TipSetStateRoot: link1State,
	}
	link2Tsas := &chain.TipSetAndState{
		TipSet:          link2,
		TipSetStateRoot: link2State,
	}
	link3Tsas := &chain.TipSetAndState{
		TipSet:          link3,
		TipSetStateRoot: link3State,
	}

	link4Tsas := &chain.TipSetAndState{
		TipSet:          link4,
		TipSetStateRoot: link4State,
	}
	th.RequirePutTsas(ctx, t, chainStore, genTsas)
	th.RequirePutTsas(ctx, t, chainStore, link1Tsas)
	th.RequirePutTsas(ctx, t, chainStore, link2Tsas)
	th.RequirePutTsas(ctx, t, chainStore, link3Tsas)
	th.RequirePutTsas(ctx, t, chainStore, link4Tsas)
}

func requireGetTsasByParentAndHeight(t *testing.T, chain chain.Store, pKey string, h uint64) []*chain.TipSetAndState {
	tsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(pKey, h)
	require.NoError(t, err)
	return tsasSlice
}

func requireHeadTipset(t *testing.T, chain chain.Store) types.TipSet {
	headTipSet, err := chain.GetTipSet(chain.GetHead())
	require.NoError(t, err)
	return *headTipSet
}

/* Putting and getting tipsets into and from the store. */

// Adding tipsets to the store doesn't error.
func TestPutTipSet(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	cs := newChainStore()
	genTsas := &chain.TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genStateRoot,
	}
	err := cs.PutTipSetAndState(ctx, genTsas)
	assert.NoError(t, err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chain := newChainStore()

	requirePutTestChain(t, chain)

	gotGTS := requireGetTipSet(ctx, t, chain, genTS.ToSortedCidSet())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, chain, genTS.ToSortedCidSet())

	got1TS := requireGetTipSet(ctx, t, chain, link1.ToSortedCidSet())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, chain, link1.ToSortedCidSet())

	got2TS := requireGetTipSet(ctx, t, chain, link2.ToSortedCidSet())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, chain, link2.ToSortedCidSet())

	got3TS := requireGetTipSet(ctx, t, chain, link3.ToSortedCidSet())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, chain, link3.ToSortedCidSet())

	got4TS := requireGetTipSet(ctx, t, chain, link4.ToSortedCidSet())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, chain, link4.ToSortedCidSet())
	assert.Equal(t, genTS, *gotGTS)
	assert.Equal(t, link1, *got1TS)
	assert.Equal(t, link2, *got2TS)
	assert.Equal(t, link3, *got3TS)
	assert.Equal(t, link4, *got4TS)

	assert.Equal(t, genStateRoot, gotGTSSR)
	assert.Equal(t, link1State, got1TSSR)
	assert.Equal(t, link2State, got2TSSR)
	assert.Equal(t, link3State, got3TSSR)
	assert.Equal(t, link4State, got4TSSR)
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chain := newChainStore()

	requirePutTestChain(t, chain)
	pkg := types.SortedCidSet{}.String() // empty cid set is genesis pIDs
	pk1 := genTS.String()
	pk2 := link1.String()
	pk3 := link2.String()
	pk4 := link3.String()

	gotG := requireGetTsasByParentAndHeight(t, chain, pkg, uint64(0))
	got1 := requireGetTsasByParentAndHeight(t, chain, pk1, uint64(1))
	got2 := requireGetTsasByParentAndHeight(t, chain, pk2, uint64(2))
	got3 := requireGetTsasByParentAndHeight(t, chain, pk3, uint64(3))
	got4 := requireGetTsasByParentAndHeight(t, chain, pk4, uint64(6)) // two null blocks in between 3 and 4!

	assert.Equal(t, genTS, gotG[0].TipSet)
	assert.Equal(t, link1, got1[0].TipSet)
	assert.Equal(t, link2, got2[0].TipSet)
	assert.Equal(t, link3, got3[0].TipSet)
	assert.Equal(t, link4, got4[0].TipSet)

	assert.Equal(t, genStateRoot, gotG[0].TipSetStateRoot)
	assert.Equal(t, link1State, got1[0].TipSetStateRoot)
	assert.Equal(t, link2State, got2[0].TipSetStateRoot)
	assert.Equal(t, link3State, got3[0].TipSetStateRoot)
	assert.Equal(t, link4State, got4[0].TipSetStateRoot)
}

func TestGetMultipleByParent(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chainStore := newChainStore()

	mockSigner, ki := types.NewMockSignersAndKeyInfo(2)
	mockSignerPubKey := ki[0].PublicKey()

	fakeChildParams := th.FakeChildParams{
		Parent:      genTS,
		GenesisCid:  genCid,
		StateRoot:   genStateRoot,
		MinerAddr:   minerAddress,
		Nonce:       uint64(5),
		Signer:      mockSigner,
		MinerPubKey: mockSignerPubKey,
	}

	requirePutTestChain(t, chainStore)
	pk1 := genTS.String()
	// give one parent multiple children and then query
	newBlk := th.RequireMkFakeChild(t, fakeChildParams)
	newChild := th.RequireNewTipSet(t, newBlk)
	newRoot := cidGetter()
	newChildTsas := &chain.TipSetAndState{
		TipSet:          newChild,
		TipSetStateRoot: newRoot,
	}
	th.RequirePutTsas(ctx, t, chainStore, newChildTsas)
	gotNew1 := requireGetTsasByParentAndHeight(t, chainStore, pk1, uint64(1))
	require.Equal(t, 2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if len(tsas.TipSet) == 1 {
			assert.Equal(t, newRoot, tsas.TipSetStateRoot)
		} else {
			assert.Equal(t, link1State, tsas.TipSetStateRoot)
		}
	}
}

// All blocks of a tipset can be retrieved after putting their wrapping tipset.
func TestGetBlocks(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chain := newChainStore()

	blks := []*types.Block{genesis, link1blk1, link1blk2, link2blk1,
		link2blk2, link2blk3, link3blk1, link4blk1, link4blk2}
	_, err := chain.GetBlock(ctx, genCid)
	assert.Error(t, err) // Get errors before put

	requirePutTestChain(t, chain)

	var cids types.SortedCidSet
	for _, blk := range blks {
		c := blk.Cid()
		(&cids).Add(c)
		gotBlk, err := chain.GetBlock(ctx, c)
		assert.NoError(t, err)
		assert.Equal(t, blk.Cid(), gotBlk.Cid())
	}
	gotBlks, err := chain.GetBlocks(ctx, cids)
	assert.NoError(t, err)
	assert.Equal(t, len(blks), len(gotBlks))
}

// chain.Store correctly indicates that is has all blocks in put tipsets
func TestHasAllBlocks(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chain := newChainStore()

	blks := []*types.Block{genesis, link1blk1, link1blk2, link2blk1,
		link2blk2, link2blk3, link3blk1, link4blk1, link4blk2}
	assert.False(t, chain.HasBlock(ctx, genCid)) // Has returns false before put

	requirePutTestChain(t, chain)

	var cids []cid.Cid
	for _, blk := range blks {
		c := blk.Cid()
		cids = append(cids, c)
		assert.True(t, chain.HasBlock(ctx, c))
	}
	assert.True(t, chain.HasAllBlocks(ctx, cids))
}

/* Head and its State is set and notified properly. */

// The constructor call sets the genesis block for the chain store.
func TestSetGenesis(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chain := newChainStore()
	requirePutTestChain(t, chain)
	require.Equal(t, genCid, chain.GenesisCid())
}

func assertSetHead(t *testing.T, chainStore chain.Store, ts types.TipSet) {
	ctx := context.Background()
	err := chainStore.SetHead(ctx, ts)
	assert.NoError(t, err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chain := newChainStore()
	requirePutTestChain(t, chain)

	// Head starts as an empty cid set
	assert.Equal(t, types.SortedCidSet{}, chain.GetHead())

	// Set Head
	assertSetHead(t, chain, genTS)
	assert.Equal(t, genTS.ToSortedCidSet(), chain.GetHead())

	// Move head forward
	assertSetHead(t, chain, link4)
	assert.Equal(t, link4.ToSortedCidSet(), chain.GetHead())

	// Move head back
	assertSetHead(t, chain, genTS)
	assert.Equal(t, genTS.ToSortedCidSet(), chain.GetHead())
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
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)
	chainStore := newChainStore()
	requirePutTestChain(t, chainStore)

	ps := chainStore.HeadEvents()
	chA := ps.Sub(chain.NewHeadTopic)
	chB := ps.Sub(chain.NewHeadTopic)

	assertSetHead(t, chainStore, genTS)
	assertSetHead(t, chainStore, link1)
	assertSetHead(t, chainStore, link2)
	assertSetHead(t, chainStore, link3)
	assertSetHead(t, chainStore, link4)
	assertSetHead(t, chainStore, link3)
	assertSetHead(t, chainStore, link2)
	assertSetHead(t, chainStore, link1)
	assertSetHead(t, chainStore, genTS)
	heads := []types.TipSet{genTS, link1, link2, link3, link4, link3,
		link2, link1, genTS}

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
	tf.BadUnitTestWithSideEffects(t)

	ctx := context.Background()
	initStoreTest(ctx, t)

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	chainStore := chain.NewDefaultStore(ds, genCid)
	requirePutTestChain(t, chainStore)
	assertSetHead(t, chainStore, genTS) // set the genesis block

	assertSetHead(t, chainStore, link4)
	chainStore.Stop()

	// rebuild chain with same datastore
	rebootChain := chain.NewDefaultStore(ds, genCid)
	err := rebootChain.Load(ctx)
	assert.NoError(t, err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootChain, link2.ToSortedCidSet())
	assert.Equal(t, link2, *got2)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(t, rebootChain, link3.String(), uint64(6))
	assert.Equal(t, 1, len(got4))
	assert.Equal(t, link4, got4[0].TipSet)

	// Check that chainStore store has blocks
	assert.True(t, rebootChain.HasBlock(ctx, link3blk1.Cid()))
	assert.True(t, rebootChain.HasBlock(ctx, link2blk3.Cid()))
	assert.True(t, rebootChain.HasBlock(ctx, genesis.Cid()))
}
