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
func initStoreTest(ctx context.Context, t *testing.T, dstP *DefaultSyncerTestParams) {
	powerTable := &th.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), powerTable, dstP.genCid, proofs.NewFakeVerifier(true, nil))
	initGenesisWrapper := func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error) {
		return initGenesis(dstP.minerAddress, dstP.minerOwnerAddress, dstP.minerPeerID, cst, bs)
	}
	initSyncTest(t, con, initGenesisWrapper, cst, bs, r, dstP)
	requireSetTestChain(t, con, true, dstP)
}

func newChainStore(dstP *DefaultSyncerTestParams) chain.Store {
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	return chain.NewDefaultStore(ds, dstP.genCid)
}

// requirePutTestChain adds all test chain tipsets to the passed in chain store.
func requirePutTestChain(t *testing.T, chainStore chain.Store, dstP *DefaultSyncerTestParams) {
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

	gotGTS := requireGetTipSet(ctx, t, chain, dstP.genTS.ToSortedCidSet())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.genTS.ToSortedCidSet())

	got1TS := requireGetTipSet(ctx, t, chain, dstP.link1.ToSortedCidSet())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link1.ToSortedCidSet())

	got2TS := requireGetTipSet(ctx, t, chain, dstP.link2.ToSortedCidSet())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link2.ToSortedCidSet())

	got3TS := requireGetTipSet(ctx, t, chain, dstP.link3.ToSortedCidSet())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link3.ToSortedCidSet())

	got4TS := requireGetTipSet(ctx, t, chain, dstP.link4.ToSortedCidSet())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, chain, dstP.link4.ToSortedCidSet())
	assert.Equal(t, dstP.genTS, *gotGTS)
	assert.Equal(t, dstP.link1, *got1TS)
	assert.Equal(t, dstP.link2, *got2TS)
	assert.Equal(t, dstP.link3, *got3TS)
	assert.Equal(t, dstP.link4, *got4TS)

	assert.Equal(t, dstP.genStateRoot, gotGTSSR)
	assert.Equal(t, dstP.link1State, got1TSSR)
	assert.Equal(t, dstP.link2State, got2TSSR)
	assert.Equal(t, dstP.link3State, got3TSSR)
	assert.Equal(t, dstP.link4State, got4TSSR)
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)

	requirePutTestChain(t, chain, dstP)
	pkg := types.SortedCidSet{}.String() // empty cid set is dstP.genesis pIDs
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
	mockSignerPubKey := ki[0].PublicKey()

	fakeChildParams := th.FakeChildParams{
		Parent:      dstP.genTS,
		GenesisCid:  dstP.genCid,
		StateRoot:   dstP.genStateRoot,
		MinerAddr:   dstP.minerAddress,
		Nonce:       uint64(5),
		Signer:      mockSigner,
		MinerPubKey: mockSignerPubKey,
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
	th.RequirePutTsas(ctx, t, chainStore, newChildTsas)
	gotNew1 := requireGetTsasByParentAndHeight(t, chainStore, pk1, uint64(1))
	require.Equal(t, 2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if len(tsas.TipSet) == 1 {
			assert.Equal(t, newRoot, tsas.TipSetStateRoot)
		} else {
			assert.Equal(t, dstP.link1State, tsas.TipSetStateRoot)
		}
	}
}

// All blocks of a tipset can be retrieved after putting their wrapping tipset.
func TestGetBlocks(t *testing.T) {
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)

	blks := []*types.Block{dstP.genesis, dstP.link1blk1, dstP.link1blk2, dstP.link2blk1,
		dstP.link2blk2, dstP.link2blk3, dstP.link3blk1, dstP.link4blk1, dstP.link4blk2}
	_, err := chain.GetBlock(ctx, dstP.genCid)
	assert.Error(t, err) // Get errors before put

	requirePutTestChain(t, chain, dstP)

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
	tf.UnitTest(t)
	dstP := initDSTParams()

	ctx := context.Background()
	initStoreTest(ctx, t, dstP)
	chain := newChainStore(dstP)

	blks := []*types.Block{dstP.genesis, dstP.link1blk1, dstP.link1blk2, dstP.link2blk1,
		dstP.link2blk2, dstP.link2blk3, dstP.link3blk1, dstP.link4blk1, dstP.link4blk2}
	assert.False(t, chain.HasBlock(ctx, dstP.genCid)) // Has returns false before put

	requirePutTestChain(t, chain, dstP)

	var cids []cid.Cid
	for _, blk := range blks {
		c := blk.Cid()
		cids = append(cids, c)
		assert.True(t, chain.HasBlock(ctx, c))
	}
	assert.True(t, chain.HasAllBlocks(ctx, cids))
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

func assertSetHead(t *testing.T, chainStore chain.Store, ts types.TipSet) {
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
	assert.Equal(t, types.SortedCidSet{}, chain.GetHead())

	// Set Head
	assertSetHead(t, chain, dstP.genTS)
	assert.Equal(t, dstP.genTS.ToSortedCidSet(), chain.GetHead())

	// Move head forward
	assertSetHead(t, chain, dstP.link4)
	assert.Equal(t, dstP.link4.ToSortedCidSet(), chain.GetHead())

	// Move head back
	assertSetHead(t, chain, dstP.genTS)
	assert.Equal(t, dstP.genTS.ToSortedCidSet(), chain.GetHead())
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

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	chainStore := chain.NewDefaultStore(ds, dstP.genCid)
	requirePutTestChain(t, chainStore, dstP)
	assertSetHead(t, chainStore, dstP.genTS) // set the genesis block

	assertSetHead(t, chainStore, dstP.link4)
	chainStore.Stop()

	// rebuild chain with same datastore
	rebootChain := chain.NewDefaultStore(ds, dstP.genCid)
	err := rebootChain.Load(ctx)
	assert.NoError(t, err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootChain, dstP.link2.ToSortedCidSet())
	assert.Equal(t, dstP.link2, *got2)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(t, rebootChain, dstP.link3.String(), uint64(6))
	assert.Equal(t, 1, len(got4))
	assert.Equal(t, dstP.link4, got4[0].TipSet)

	// Check that chainStore store has blocks
	assert.True(t, rebootChain.HasBlock(ctx, dstP.link3blk1.Cid()))
	assert.True(t, rebootChain.HasBlock(ctx, dstP.link2blk3.Cid()))
	assert.True(t, rebootChain.HasBlock(ctx, dstP.genesis.Cid()))
}
