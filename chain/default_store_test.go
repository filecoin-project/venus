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
	"github.com/filecoin-project/go-filecoin/types"
)

// Note: many of these tests use the test chain defined in the init function of default_syncer_test.
func initStoreTest(ctx context.Context, require *require.Assertions) {
	powerTable := &th.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, th.NewTestProcessor(), powerTable, genCid, proofs.NewFakeVerifier(true, nil))
	initSyncTest(require, con, initGenesis, cst, bs, r)
	requireSetTestChain(require, con, true)
}

func newChainStore() chain.Store {
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	return chain.NewDefaultStore(ds, hamt.NewCborStore(), genCid)
}

// requirePutTestChain adds all test chain tipsets to the passed in chain store.
func requirePutTestChain(require *require.Assertions, chainStore chain.Store) {
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
	th.RequirePutTsas(ctx, require, chainStore, genTsas)
	th.RequirePutTsas(ctx, require, chainStore, link1Tsas)
	th.RequirePutTsas(ctx, require, chainStore, link2Tsas)
	th.RequirePutTsas(ctx, require, chainStore, link3Tsas)
	th.RequirePutTsas(ctx, require, chainStore, link4Tsas)
}

func requireGetTsasByParentAndHeight(ctx context.Context, require *require.Assertions, chain chain.Store, pKey string, h uint64) []*chain.TipSetAndState {
	tsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(ctx, pKey, h)
	require.NoError(err)
	return tsasSlice
}

/* Putting and getting tipsets into and from the store. */

// Adding tipsets to the store doesn't error.
func TestPutTipSet(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	assert := assert.New(t)
	cs := newChainStore()
	genTsas := &chain.TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genStateRoot,
	}
	err := cs.PutTipSetAndState(ctx, genTsas)
	assert.NoError(err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()

	requirePutTestChain(require, chain)

	gotG := requireGetTsas(ctx, require, chain, genTS.ToSortedCidSet())
	got1 := requireGetTsas(ctx, require, chain, link1.ToSortedCidSet())
	got2 := requireGetTsas(ctx, require, chain, link2.ToSortedCidSet())
	got3 := requireGetTsas(ctx, require, chain, link3.ToSortedCidSet())
	got4 := requireGetTsas(ctx, require, chain, link4.ToSortedCidSet())

	assert.Equal(genTS, gotG.TipSet)
	assert.Equal(link1, got1.TipSet)
	assert.Equal(link2, got2.TipSet)
	assert.Equal(link3, got3.TipSet)
	assert.Equal(link4, got4.TipSet)

	assert.Equal(genStateRoot, gotG.TipSetStateRoot)
	assert.Equal(link1State, got1.TipSetStateRoot)
	assert.Equal(link2State, got2.TipSetStateRoot)
	assert.Equal(link3State, got3.TipSetStateRoot)
	assert.Equal(link4State, got4.TipSetStateRoot)
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()

	requirePutTestChain(require, chain)
	pkg := types.SortedCidSet{}.String() // empty cid set is genesis pIDs
	pk1 := genTS.String()
	pk2 := link1.String()
	pk3 := link2.String()
	pk4 := link3.String()

	gotG := requireGetTsasByParentAndHeight(ctx, require, chain, pkg, uint64(0))
	got1 := requireGetTsasByParentAndHeight(ctx, require, chain, pk1, uint64(1))
	got2 := requireGetTsasByParentAndHeight(ctx, require, chain, pk2, uint64(2))
	got3 := requireGetTsasByParentAndHeight(ctx, require, chain, pk3, uint64(3))
	got4 := requireGetTsasByParentAndHeight(ctx, require, chain, pk4, uint64(6)) // two null blocks in between 3 and 4!

	assert.Equal(genTS, gotG[0].TipSet)
	assert.Equal(link1, got1[0].TipSet)
	assert.Equal(link2, got2[0].TipSet)
	assert.Equal(link3, got3[0].TipSet)
	assert.Equal(link4, got4[0].TipSet)

	assert.Equal(genStateRoot, gotG[0].TipSetStateRoot)
	assert.Equal(link1State, got1[0].TipSetStateRoot)
	assert.Equal(link2State, got2[0].TipSetStateRoot)
	assert.Equal(link3State, got3[0].TipSetStateRoot)
	assert.Equal(link4State, got4[0].TipSetStateRoot)
}

func TestGetMultipleByParent(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
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

	requirePutTestChain(require, chainStore)
	pk1 := genTS.String()
	// give one parent multiple children and then query
	newBlk := th.RequireMkFakeChild(require, fakeChildParams)
	newChild := th.RequireNewTipSet(require, newBlk)
	newRoot := cidGetter()
	newChildTsas := &chain.TipSetAndState{
		TipSet:          newChild,
		TipSetStateRoot: newRoot,
	}
	th.RequirePutTsas(ctx, require, chainStore, newChildTsas)
	gotNew1 := requireGetTsasByParentAndHeight(ctx, require, chainStore, pk1, uint64(1))
	require.Equal(2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if len(tsas.TipSet) == 1 {
			assert.Equal(newRoot, tsas.TipSetStateRoot)
		} else {
			assert.Equal(link1State, tsas.TipSetStateRoot)
		}
	}
}

// All blocks of a tipset can be retrieved after putting their wrapping tipset.
func TestGetBlocks(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()

	blks := []*types.Block{genesis, link1blk1, link1blk2, link2blk1,
		link2blk2, link2blk3, link3blk1, link4blk1, link4blk2}
	_, err := chain.GetBlock(ctx, genCid)
	assert.Error(err) // Get errors before put

	requirePutTestChain(require, chain)

	var cids types.SortedCidSet
	for _, blk := range blks {
		c := blk.Cid()
		(&cids).Add(c)
		gotBlk, err := chain.GetBlock(ctx, c)
		assert.NoError(err)
		assert.Equal(blk.Cid(), gotBlk.Cid())
	}
	gotBlks, err := chain.GetBlocks(ctx, cids)
	assert.NoError(err)
	assert.Equal(len(blks), len(gotBlks))
}

// chain.Store correctly indicates that is has all blocks in put tipsets
func TestHasAllBlocks(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()

	blks := []*types.Block{genesis, link1blk1, link1blk2, link2blk1,
		link2blk2, link2blk3, link3blk1, link4blk1, link4blk2}
	assert.False(chain.HasBlock(ctx, genCid)) // Has returns false before put

	requirePutTestChain(require, chain)

	var cids []cid.Cid
	for _, blk := range blks {
		c := blk.Cid()
		cids = append(cids, c)
		assert.True(chain.HasBlock(ctx, c))
	}
	assert.True(chain.HasAllBlocks(ctx, cids))
}

/* Head and its State is set and notified properly. */

// The constructor call sets the genesis block for the chain store.
func TestSetGenesis(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	chain := newChainStore()
	requirePutTestChain(require, chain)
	require.Equal(genCid, chain.GenesisCid())
}

func assertSetHead(assert *assert.Assertions, chainStore chain.Store, ts types.TipSet) {
	ctx := context.Background()
	err := chainStore.SetHead(ctx, ts)
	assert.NoError(err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()
	requirePutTestChain(require, chain)

	// Head starts as nil
	assert.Equal(types.SortedCidSet{}, chain.GetHead())

	// Set Head
	assertSetHead(assert, chain, genTS)
	assert.Equal(genTS.ToSortedCidSet(), chain.GetHead())

	// Move head forward
	assertSetHead(assert, chain, link4)
	assert.Equal(link4.ToSortedCidSet(), chain.GetHead())

	// Move head back
	assertSetHead(assert, chain, genTS)
	assert.Equal(genTS.ToSortedCidSet(), chain.GetHead())
}

func assertEmptyCh(assert *assert.Assertions, ch <-chan interface{}) {
	select {
	case <-ch:
		assert.True(false)
	default:
	}
}

// Head events are propagated on HeadEvents.
func TestHeadEvents(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chainStore := newChainStore()
	requirePutTestChain(require, chainStore)

	ps := chainStore.HeadEvents()
	chA := ps.Sub(chain.NewHeadTopic)
	chB := ps.Sub(chain.NewHeadTopic)

	assertSetHead(assert, chainStore, genTS)
	assertSetHead(assert, chainStore, link1)
	assertSetHead(assert, chainStore, link2)
	assertSetHead(assert, chainStore, link3)
	assertSetHead(assert, chainStore, link4)
	assertSetHead(assert, chainStore, link3)
	assertSetHead(assert, chainStore, link2)
	assertSetHead(assert, chainStore, link1)
	assertSetHead(assert, chainStore, genTS)
	heads := []types.TipSet{genTS, link1, link2, link3, link4, link3,
		link2, link1, genTS}

	// Heads arrive in the expected order
	for i := 0; i < 9; i++ {
		headA := <-chA
		headB := <-chB
		assert.Equal(headA, headB)
		assert.Equal(headA, heads[i])
	}

	// No extra notifications
	assertEmptyCh(assert, chA)
	assertEmptyCh(assert, chB)
}

/* Loading  */
// Load does not error and gives the chain store access to all blocks and
// tipset indexes along the heaviest chain.
func TestLoadAndReboot(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	assert := assert.New(t)
	require := require.New(t)

	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	chainStore := chain.NewDefaultStore(ds, hamt.NewCborStore(), genCid)
	requirePutTestChain(require, chainStore)
	assertSetHead(assert, chainStore, genTS) // set the genesis block

	assertSetHead(assert, chainStore, link4)
	chainStore.Stop()

	// rebuild chain with same datastore
	rebootChain := chain.NewDefaultStore(ds, hamt.NewCborStore(), genCid)
	err := rebootChain.Load(ctx)
	assert.NoError(err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTsas(ctx, require, rebootChain, link2.ToSortedCidSet())
	assert.Equal(link2, got2.TipSet)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(ctx, require, rebootChain, link3.String(), uint64(6))
	assert.Equal(1, len(got4))
	assert.Equal(link4, got4[0].TipSet)

	// Check that chainStore store has blocks
	assert.True(rebootChain.HasBlock(ctx, link3blk1.Cid()))
	assert.True(rebootChain.HasBlock(ctx, link2blk3.Cid()))
	assert.True(rebootChain.HasBlock(ctx, genesis.Cid()))
}
