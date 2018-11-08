package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

var (
	// Chain diagram below.  Note that blocks in the same tipset are in parentheses.
	//
	// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (link4blk1, link4blk2)

	// Blocks
	genesis                         *types.Block
	link1blk1, link1blk2            *types.Block
	link2blk1, link2blk2, link2blk3 *types.Block
	link3blk1                       *types.Block
	link4blk1, link4blk2            *types.Block

	// Cids
	genCid                                                       *cid.Cid
	genStateRoot, link1State, link2State, link3State, link4State *cid.Cid

	// TipSets
	genTS, link1, link2, link3, link4 consensus.TipSet

	// utils
	cidGetter func() *cid.Cid
)

func init() {
	// Set up the test chain
	bs := bstore.NewBlockstore(repo.NewInMemoryRepo().Datastore())
	cst := hamt.NewCborStore()
	var err error
	genesis, err = consensus.InitGenesis(cst, bs)
	if err != nil {
		panic(err)
	}
	genCid = genesis.Cid()
	genTS = MustNewTipSet(genesis)

	// mock state root cids
	cidGetter = types.NewCidForTestGetter()

	genStateRoot = genesis.StateRoot
}

// This function sets global variables according to the tests needs.  The
// test chain's basic structure is always the same, but some tests want
// mocked stateRoots or parent weight calculations from different consensus protocols.
func requireSetTestChain(require *require.Assertions, con consensus.Protocol, mockStateRoots bool) {
	link1blk1 = RequireMkFakeChildWithCon(require, genTS, genCid, genStateRoot, uint64(0), uint64(0), con)
	link1blk2 = RequireMkFakeChildWithCon(require, genTS, genCid, genStateRoot, uint64(1), uint64(0), con)
	link1 = consensus.RequireNewTipSet(require, link1blk1, link1blk2)

	if mockStateRoots {
		link1State = cidGetter()
	} else {
		link1State = genStateRoot
	}
	link2blk1 = RequireMkFakeChildWithCon(require, link1, genCid, link1State, uint64(0), uint64(0), con)
	link2blk2 = RequireMkFakeChildWithCon(require, link1, genCid, link1State, uint64(1), uint64(0), con)
	link2blk3 = RequireMkFakeChildWithCon(require, link1, genCid, link1State, uint64(2), uint64(0), con)
	link2 = consensus.RequireNewTipSet(require, link2blk1, link2blk2, link2blk3)

	if mockStateRoots {
		link2State = cidGetter()
	} else {
		link2State = genStateRoot
	}
	link3blk1 = RequireMkFakeChildWithCon(require, link2, genCid, link2State, uint64(0), uint64(0), con)
	link3 = consensus.RequireNewTipSet(require, link3blk1)

	if mockStateRoots {
		link3State = cidGetter()
	} else {
		link3State = genStateRoot
	}

	link4blk1 = RequireMkFakeChildWithCon(require, link3, genCid, link3State, uint64(0), uint64(2), con) // 2 null blks between link 3 and 4
	link4blk2 = RequireMkFakeChildWithCon(require, link3, genCid, link3State, uint64(1), uint64(2), con)
	link4 = consensus.RequireNewTipSet(require, link4blk1, link4blk2)

	if mockStateRoots {
		link4State = cidGetter()
	} else {
		link4State = genStateRoot
	}
}

// loadSyncerFromRepo creates a store and syncer from an existing repo.
func loadSyncerFromRepo(require *require.Assertions, r repo.Repo) (Syncer, *hamt.CborIpldStore) {
	powerTable := &consensus.TestView{}
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, powerTable, genCid)
	syncer, chain, cst, _ := initSyncTest(require, con, consensus.InitGenesis, cst, bs, r)
	ctx := context.Background()
	err := chain.Load(ctx)
	require.NoError(err)
	return syncer, cst
}

// initSyncTestDefault creates and returns the datastructures (chain store, syncer, etc)
// needed to run tests.  It also sets the global test variables appropriately.
func initSyncTestDefault(require *require.Assertions) (Syncer, Store, *hamt.CborIpldStore, repo.Repo) {
	powerTable := &consensus.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, powerTable, genCid)
	requireSetTestChain(require, con, false)
	return initSyncTest(require, con, consensus.InitGenesis, cst, bs, r)
}

// initSyncTestWithPowerTable creates and returns the datastructures (chain store, syncer, etc)
// needed to run tests.  It also sets the global test variables appropriately.
func initSyncTestWithPowerTable(require *require.Assertions, powerTable consensus.PowerTableView) (Syncer, Store, *hamt.CborIpldStore, consensus.Protocol) {
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, powerTable, genCid)
	requireSetTestChain(require, con, false)
	sync, chain, cst, _ := initSyncTest(require, con, consensus.InitGenesis, cst, bs, r)
	return sync, chain, cst, con
}

func initSyncTest(require *require.Assertions, con consensus.Protocol, genFunc func(cst *hamt.CborIpldStore, bs bstore.Blockstore) (*types.Block, error), cst *hamt.CborIpldStore, bs bstore.Blockstore, r repo.Repo) (Syncer, Store, *hamt.CborIpldStore, repo.Repo) {
	ctx := context.Background()

	// chain.Store
	calcGenBlk, err := genFunc(cst, bs) // flushes state
	require.NoError(err)
	chainDS := r.ChainDatastore()
	chain := NewDefaultStore(chainDS, cst, calcGenBlk.Cid())

	// chain.Syncer
	syncer := NewDefaultSyncer(cst, cst, con, chain) // note we use same cst for on and offline for tests

	// Initialize stores to contain genesis block and state
	calcGenTS := consensus.RequireNewTipSet(require, calcGenBlk)
	genTsas := &TipSetAndState{
		TipSet:          calcGenTS,
		TipSetStateRoot: genStateRoot,
	}
	RequirePutTsas(ctx, require, chain, genTsas)
	err = chain.SetHead(ctx, calcGenTS) // Initialize chain store with correct genesis
	require.NoError(err)
	requireHead(require, chain, calcGenTS)
	requireTsAdded(require, chain, calcGenTS)

	return syncer, chain, cst, r
}

func containsTipSet(tsasSlice []*TipSetAndState, ts consensus.TipSet) bool {
	for _, tsas := range tsasSlice {
		if tsas.TipSet.String() == ts.String() { //bingo
			return true
		}
	}
	return false
}

func requireTsAdded(require *require.Assertions, chain Store, ts consensus.TipSet) {
	ctx := context.Background()
	h, err := ts.Height()
	require.NoError(err)
	// Tip Index correctly updated
	gotTsas, err := chain.GetTipSetAndState(ctx, ts.String())
	require.NoError(err)
	require.Equal(ts, gotTsas.TipSet)
	parent, err := ts.Parents()
	require.NoError(err)
	childTsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(ctx, parent.String(), h)
	require.NoError(err)
	require.True(containsTipSet(childTsasSlice, ts))

	// Blocks exist in store
	for _, blk := range ts {
		require.True(chain.HasBlock(ctx, blk.Cid()))
	}
}

func assertTsAdded(assert *assert.Assertions, chain Store, ts consensus.TipSet) {
	ctx := context.Background()
	h, err := ts.Height()
	assert.NoError(err)
	// Tip Index correctly updated
	gotTsas, err := chain.GetTipSetAndState(ctx, ts.String())
	assert.NoError(err)
	assert.Equal(ts, gotTsas.TipSet)
	parent, err := ts.Parents()
	assert.NoError(err)
	childTsasSlice, err := chain.GetTipSetAndStatesByParentsAndHeight(ctx, parent.String(), h)
	assert.NoError(err)
	assert.True(containsTipSet(childTsasSlice, ts))

	// Blocks exist in store
	for _, blk := range ts {
		assert.True(chain.HasBlock(ctx, blk.Cid()))
	}
}

func assertNoAdd(assert *assert.Assertions, chain Store, cids []*cid.Cid) {
	ctx := context.Background()
	// Tip Index correctly updated
	_, err := chain.GetTipSetAndState(ctx, types.NewSortedCidSet(cids...).String())
	assert.Error(err)
	// Blocks exist in store
	for _, c := range cids {
		assert.False(chain.HasBlock(ctx, c))
	}
}

func requireHead(require *require.Assertions, chain Store, head consensus.TipSet) {
	gotHead := chain.Head()
	require.Equal(head, gotHead)
}

func assertHead(assert *assert.Assertions, chain Store, head consensus.TipSet) {
	gotHead := chain.Head()
	assert.Equal(head, gotHead)
}

func requirePutBlocks(require *require.Assertions, cst *hamt.CborIpldStore, blks ...*types.Block) []*cid.Cid {
	ctx := context.Background()
	var cids []*cid.Cid
	for _, blk := range blks {
		c, err := cst.Put(ctx, blk)
		require.NoError(err)
		cids = append(cids, c)
	}
	return cids
}

/* Regular Degular syncing */

// Syncer syncs a single block
func TestSyncOneBlock(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()
	expectedTs := consensus.RequireNewTipSet(require, link1blk1)

	cids := requirePutBlocks(require, cst, link1blk1)
	err := syncer.HandleNewBlocks(ctx, cids)
	assert.NoError(err)

	assertTsAdded(assert, chain, expectedTs)
	assertHead(assert, chain, expectedTs)
}

// Syncer syncs a single tipset.
func TestSyncOneTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	cids := requirePutBlocks(require, cst, link1blk1, link1blk2)
	err := syncer.HandleNewBlocks(ctx, cids)
	assert.NoError(err)

	assertTsAdded(assert, chain, link1)
	assertHead(assert, chain, link1)
}

// Syncer syncs one tipset, block by block.
func TestSyncTipSetBlockByBlock(t *testing.T) {
	pt := &powerTableForWidenTest{}
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestWithPowerTable(require, pt)
	ctx := context.Background()
	expTs1 := consensus.RequireNewTipSet(require, link1blk1)

	cids := requirePutBlocks(require, cst, link1blk1, link1blk2)
	err := syncer.HandleNewBlocks(ctx, []*cid.Cid{cids[0]})
	assert.NoError(err)

	assertTsAdded(assert, chain, expTs1)
	assertHead(assert, chain, expTs1)

	err = syncer.HandleNewBlocks(ctx, []*cid.Cid{cids[1]})
	assert.NoError(err)

	assertTsAdded(assert, chain, link1)
	assertHead(assert, chain, link1)
}

// Syncer syncs a chain, tipset by tipset.
func TestSyncChainTipSetByTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	cids1 := requirePutBlocks(require, cst, link1.ToSlice()...)
	cids2 := requirePutBlocks(require, cst, link2.ToSlice()...)
	cids3 := requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	err := syncer.HandleNewBlocks(ctx, cids1)
	assert.NoError(err)
	assertTsAdded(assert, chain, link1)
	assertHead(assert, chain, link1)

	err = syncer.HandleNewBlocks(ctx, cids2)
	assert.NoError(err)
	assertTsAdded(assert, chain, link2)
	assertHead(assert, chain, link2)

	err = syncer.HandleNewBlocks(ctx, cids3)
	assert.NoError(err)
	assertTsAdded(assert, chain, link3)
	assertHead(assert, chain, link3)

	err = syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chain, link4)
	assertHead(assert, chain, link4)
}

// Syncer syncs a whole chain given only the head cids.
func TestSyncChainHead(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	err := syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chain, link4)
	assertTsAdded(assert, chain, link3)
	assertTsAdded(assert, chain, link2)
	assertTsAdded(assert, chain, link1)
	assertHead(assert, chain, link4)
}

// Syncer determines the heavier fork.
func TestSyncIgnoreLightFork(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	forkbase := consensus.RequireNewTipSet(require, link2blk1)
	forkblk1 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(0), uint64(0))
	forklink1 := consensus.RequireNewTipSet(require, forkblk1)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	forkCids1 := requirePutBlocks(require, cst, forklink1.ToSlice()...)

	// Sync heaviest branch first.
	err := syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chain, link4)
	assertHead(assert, chain, link4)

	// lighter fork should be processed but not change head.
	assert.NoError(syncer.HandleNewBlocks(ctx, forkCids1))
	assertTsAdded(assert, chain, forklink1)
	assertHead(assert, chain, link4)
}

// Correctly sync a heavier fork
func TestHeavierFork(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	forkbase := consensus.RequireNewTipSet(require, link2blk1)
	forklink1blk1 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(0), uint64(0))
	forklink1blk2 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(1), uint64(0))
	forklink1blk3 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(2), uint64(0))
	forklink1 := consensus.RequireNewTipSet(require, forklink1blk1, forklink1blk2, forklink1blk3)

	forklink2blk1 := RequireMkFakeChild(require, forklink1, genCid, genStateRoot, uint64(0), uint64(0))
	forklink2blk2 := RequireMkFakeChild(require, forklink1, genCid, genStateRoot, uint64(1), uint64(0))
	forklink2blk3 := RequireMkFakeChild(require, forklink1, genCid, genStateRoot, uint64(2), uint64(0))
	forklink2 := consensus.RequireNewTipSet(require, forklink2blk1, forklink2blk2, forklink2blk3)

	forklink3blk1 := RequireMkFakeChild(require, forklink2, genCid, genStateRoot, uint64(0), uint64(0))
	forklink3blk2 := RequireMkFakeChild(require, forklink2, genCid, genStateRoot, uint64(1), uint64(0))
	forklink3 := consensus.RequireNewTipSet(require, forklink3blk1, forklink3blk2)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)
	_ = requirePutBlocks(require, cst, forklink1.ToSlice()...)
	_ = requirePutBlocks(require, cst, forklink2.ToSlice()...)
	forkHead := requirePutBlocks(require, cst, forklink3.ToSlice()...)

	err := syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chain, link4)
	assertHead(assert, chain, link4)

	// heavier fork updates head
	err = syncer.HandleNewBlocks(ctx, forkHead)
	assert.NoError(err)
	assertTsAdded(assert, chain, forklink1)
	assertTsAdded(assert, chain, forklink2)
	assertTsAdded(assert, chain, forklink3)
	assertHead(assert, chain, forklink3)
}

// Syncer errors if blocks don't form a tipset
func TestBlocksNotATipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	badCids := []*cid.Cid{link1blk1.Cid(), link2blk1.Cid()}
	err := syncer.HandleNewBlocks(ctx, badCids)
	assert.Error(err)
	assertNoAdd(assert, chain, badCids)
}

/* particularly tricky edge cases relating to subtle Expected Consensus requirements */

// Syncer is capable of recovering from a fork reorg after Load.
func TestLoadFork(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, r := initSyncTestDefault(require)
	ctx := context.Background()

	// Set up chain store to have standard chain up to link2
	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	cids2 := requirePutBlocks(require, cst, link2.ToSlice()...)
	err := syncer.HandleNewBlocks(ctx, cids2)
	require.NoError(err)

	// Now sync the store with a heavier fork, forking off link1.
	forkbase := consensus.RequireNewTipSet(require, link2blk1)
	forklink1blk1 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(0), uint64(0))
	forklink1blk2 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(1), uint64(0))
	forklink1blk3 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(2), uint64(0))
	forklink1 := consensus.RequireNewTipSet(require, forklink1blk1, forklink1blk2, forklink1blk3)

	forklink2blk1 := RequireMkFakeChild(require, forklink1, genCid, genStateRoot, uint64(0), uint64(0))
	forklink2blk2 := RequireMkFakeChild(require, forklink1, genCid, genStateRoot, uint64(1), uint64(0))
	forklink2blk3 := RequireMkFakeChild(require, forklink1, genCid, genStateRoot, uint64(2), uint64(0))
	forklink2 := consensus.RequireNewTipSet(require, forklink2blk1, forklink2blk2, forklink2blk3)

	forklink3blk1 := RequireMkFakeChild(require, forklink2, genCid, genStateRoot, uint64(0), uint64(0))
	forklink3blk2 := RequireMkFakeChild(require, forklink2, genCid, genStateRoot, uint64(1), uint64(0))
	forklink3 := consensus.RequireNewTipSet(require, forklink3blk1, forklink3blk2)

	_ = requirePutBlocks(require, cst, forklink1.ToSlice()...)
	_ = requirePutBlocks(require, cst, forklink2.ToSlice()...)
	forkHead := requirePutBlocks(require, cst, forklink3.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, forkHead)
	require.NoError(err)
	requireHead(require, chain, forklink3)

	// Shut down store, reload and wire to syncer.
	loadSyncer, loadCst := loadSyncerFromRepo(require, r)

	// Test that the syncer can sync a block on the old chain
	cids3 := requirePutBlocks(require, loadCst, link3.ToSlice()...)
	err = loadSyncer.HandleNewBlocks(ctx, cids3)
	assert.NoError(err)
}

// Syncer must track state of subsets of parent tipsets tracked in the store
// when they are the ancestor in a chain.  This is in order to maintain the
// invariant that the aggregate state of the  parents of the base of a collected chain
// is kept in the store.  This invariant allows chains built on subsets of
// tracked tipsets to be handled correctly.
// This test tests that the syncer stores the state of such a base tipset of a collected chain,
// i.e. a subset of an existing tipset in the store.
//
// Ex: {A1, A2} -> {B1, B2, B3} in store to start
// {B1, B2} -> {C1, C2} chain 1 input to syncer
// C1 -> D1 chain 2 input to syncer
//
// The last operation will fail if the state of subset {B1, B2} is not
// kept in the store because syncing C1 requires retrieving parent state.
func TestSubsetParent(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, _, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()

	// Set up store to have standard chain up to link2
	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	cids2 := requirePutBlocks(require, cst, link2.ToSlice()...)
	err := syncer.HandleNewBlocks(ctx, cids2)
	require.NoError(err)

	// Sync one tipset with a parent equal to a subset of an existing
	// tipset in the store.
	forkbase := consensus.RequireNewTipSet(require, link2blk1, link2blk2)
	forkblk1 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(0), uint64(0))
	forkblk2 := RequireMkFakeChild(require, forkbase, genCid, genStateRoot, uint64(1), uint64(0))
	forklink := consensus.RequireNewTipSet(require, forkblk1, forkblk2)
	forkHead := requirePutBlocks(require, cst, forklink.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, forkHead)
	assert.NoError(err)

	// Sync another tipset with a parent equal to a subset of the tipset
	// just synced.
	newForkbase := consensus.RequireNewTipSet(require, forkblk1, forkblk2)
	newForkblk := RequireMkFakeChild(require, newForkbase, genCid, genStateRoot, uint64(0), uint64(0))
	newForklink := consensus.RequireNewTipSet(require, newForkblk)
	newForkHead := requirePutBlocks(require, cst, newForklink.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, newForkHead)
	assert.NoError(err)
}

// Check that the syncer correctly adds widened chain ancestors to the store.
func TestWidenChainAncestor(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, _ := initSyncTestDefault(require)
	ctx := context.Background()
	link2blkother := RequireMkFakeChild(require, link1, genCid, genStateRoot, uint64(27), uint64(0))

	link2intersect := consensus.RequireNewTipSet(require, link2blk1, link2blkother)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	cids4 := requirePutBlocks(require, cst, link4.ToSlice()...)

	intersectCids := requirePutBlocks(require, cst, link2intersect.ToSlice()...)

	// Sync the subset of link2 first
	err := syncer.HandleNewBlocks(ctx, intersectCids)
	assert.NoError(err)
	assertTsAdded(assert, chain, link2intersect)
	assertHead(assert, chain, link2intersect)

	// Sync chain with head at link4
	err = syncer.HandleNewBlocks(ctx, cids4)
	assert.NoError(err)
	assertTsAdded(assert, chain, link4)
	assertHead(assert, chain, link4)

	// Check that the widened tipset (link2intersect U link2) is tracked
	link2Union := consensus.RequireNewTipSet(require, link2blk1, link2blk2, link2blk3, link2blkother)
	assertTsAdded(assert, chain, link2Union)
}

type powerTableForWidenTest struct{}

func (pt *powerTableForWidenTest) Total(ctx context.Context, st state.Tree, bs bstore.Blockstore) (uint64, error) {
	return uint64(100), nil
}

func (pt *powerTableForWidenTest) Miner(ctx context.Context, st state.Tree, bs bstore.Blockstore, mAddr address.Address) (uint64, error) {
	return uint64(25), nil
}

func (pt *powerTableForWidenTest) HasPower(ctx context.Context, st state.Tree, bs bstore.Blockstore, mAddr address.Address) bool {
	return true
}

// Syncer finds a heaviest tipset by combining blocks from the ancestors of a
// chain and blocks already in the store.
//
// A guide to this test -- the point is that sometimes when merging chains the syncer
// will find a new heaviest tipset that is not the head of either chain.  The syncer
// should correctly set this tipset as the head.
//
// From above we have the test-chain:
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (link4blk1, link4blk2)
//
// Now we introduce a disjoint fork on top of link1
// genesis -> (link1blk1, link1blk2) -> (forklink2blk1, forklink2blk2, forklink2blk3, forklink3blk4) -> forklink3blk1
//
// Using the provided powertable all new tipsets contribute to the weight: + 35*(num of blocks in tipset).
// So, the weight of the  head of the test chain =
//   W(link1) + 105 + 35 + 70 = W(link1) + 210 = 280
// and the weight of the head of the fork chain =
//   W(link1) + 140 + 35 = W(link1) + 175 = 245
// and the weight of the union of link2 of both branches (a valid tipset) is
//   W(link1) + 245 = 315
//
// Therefore the syncer should set the head of the store to the union of the links..
func TestHeaviestIsWidenedAncestor(t *testing.T) {
	pt := &powerTableForWidenTest{}
	assert := assert.New(t)
	require := require.New(t)
	syncer, chain, cst, con := initSyncTestWithPowerTable(require, pt)
	ctx := context.Background()

	forklink2blk1 := RequireMkFakeChildWithCon(require, link1, genCid, genStateRoot, uint64(51), uint64(0), con)
	forklink2blk2 := RequireMkFakeChildWithCon(require, link1, genCid, genStateRoot, uint64(52), uint64(0), con)
	forklink2blk3 := RequireMkFakeChildWithCon(require, link1, genCid, genStateRoot, uint64(53), uint64(0), con)
	forklink2blk4 := RequireMkFakeChildWithCon(require, link1, genCid, genStateRoot, uint64(54), uint64(0), con)
	forklink2 := consensus.RequireNewTipSet(require, forklink2blk1, forklink2blk2, forklink2blk3, forklink2blk4)

	forklink3blk1 := RequireMkFakeChildWithCon(require, forklink2, genCid, genStateRoot, uint64(0), uint64(0), con)
	forklink3 := consensus.RequireNewTipSet(require, forklink3blk1)

	_ = requirePutBlocks(require, cst, link1.ToSlice()...)
	_ = requirePutBlocks(require, cst, link2.ToSlice()...)
	_ = requirePutBlocks(require, cst, link3.ToSlice()...)
	testhead := requirePutBlocks(require, cst, link4.ToSlice()...)

	_ = requirePutBlocks(require, cst, forklink2.ToSlice()...)
	forkhead := requirePutBlocks(require, cst, forklink3.ToSlice()...)

	// Put testhead
	err := syncer.HandleNewBlocks(ctx, testhead)
	assert.NoError(err)

	// Put forkhead
	err = syncer.HandleNewBlocks(ctx, forkhead)
	assert.NoError(err)

	// Assert that widened chain is the new head
	wideTs := consensus.RequireNewTipSet(require, link2blk1, link2blk2, link2blk3, forklink2blk1, forklink2blk2, forklink2blk3, forklink2blk4)
	assertTsAdded(assert, chain, wideTs)
	assertHead(assert, chain, wideTs)
}

/* Tests with Unmocked state */

// Syncer handles MarketView weight comparisons.
// Current issue: when creating miner mining with addr0, addr0's storage head isn't found in the blockstore
// and I can't figure out why because we pass in the correct blockstore to createminerwithpower.

func TestTipSetWeightDeep(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()

	ctx := context.Background()

	// set up genesis block with power
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	testAddress := mockSigner.Addresses[0]

	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)

	weightDeepGenBlk, err := testGen(cst, bs)
	require.NoError(err)
	weightDeepGenCid := weightDeepGenBlk.Cid()
	weightDeepTS := consensus.RequireNewTipSet(require, weightDeepGenBlk)
	weightDeepStateRoot := weightDeepGenBlk.StateRoot

	con := consensus.NewExpected(cst, bs, &consensus.TestView{}, weightDeepGenCid)
	syncer, chain, cst, _ := initSyncTest(require, con, testGen, cst, bs, r)

	genTsas := &TipSetAndState{
		TipSet:          weightDeepTS,
		TipSetStateRoot: weightDeepStateRoot,
	}
	RequirePutTsas(ctx, require, chain, genTsas)
	err = chain.SetHead(ctx, weightDeepTS) // Initialize chain store with correct genesis
	require.NoError(err)
	requireHead(require, chain, weightDeepTS)
	requireTsAdded(require, chain, weightDeepTS)

	// Bootstrap the storage market using a syncer with consensus using a
	// TestView.
	// pwr1, pwr2 = 1/100. pwr3 = 98/100.
	pwr1, pwr2, pwr3 := uint64(10), uint64(10), uint64(980)

	addr0, block, nonce, err := CreateMinerWithPower(ctx, t, syncer, weightDeepGenBlk, mockSigner, 0, mockSigner.Addresses[0], uint64(0), cst, bs, weightDeepGenCid)
	require.NoError(err)
	addr1, block, nonce, err := CreateMinerWithPower(ctx, t, syncer, block, mockSigner, nonce, addr0, pwr1, cst, bs, weightDeepGenCid)
	require.NoError(err)
	addr2, block, nonce, err := CreateMinerWithPower(ctx, t, syncer, block, mockSigner, nonce, addr0, pwr2, cst, bs, weightDeepGenCid)
	require.NoError(err)
	addr3, _, _, err := CreateMinerWithPower(ctx, t, syncer, block, mockSigner, nonce, addr0, pwr3, cst, bs, weightDeepGenCid)
	require.NoError(err)

	// Now sync the chain with consensus using a MarketView.
	con = consensus.NewExpected(cst, bs, &consensus.MarketView{}, weightDeepGenCid)
	syncer = NewDefaultSyncer(cst, cst, con, chain)
	baseTS := chain.Head() // this is the last block of the bootstrapping chain creating miners
	require.Equal(1, len(baseTS))
	bootstrapStateRoot := baseTS.ToSlice()[0].StateRoot
	baseParent, err := baseTS.Parents()
	require.NoError(err)
	parentID := baseParent.String()
	parentTsas := requireGetTsas(ctx, require, chain, parentID)
	pSt, err := state.LoadStateTree(ctx, cst, parentTsas.TipSetStateRoot, builtin.Actors)
	require.NoError(err)
	bootstrapSt, err := state.LoadStateTree(ctx, cst, bootstrapStateRoot, builtin.Actors)
	require.NoError(err)
	/* Test chain diagram and weight calcs */
	// (Note f1b1 = fork 1 block 1)
	//
	// f1b1 -> {f1b2a, f1b2b}
	//
	// f2b1 -> f2b2
	//
	//  sw=starting weight, apw=added parent weight, mw=miner weight, ew=expected weight
	//  w({blk})          = sw + apw + mw      = sw + ew
	//  w({fXb1})         = sw + 0   + 11      = sw + 11
	//  w({f1b1, f2b1})   = sw + 0   + 11 * 2  = sw + 22
	//  w({f1b2a, f1b2b}) = sw + 11  + 11 * 2  = sw + 33
	//  w({f2b2})         = sw + 11  + 108 	   = sw + 119
	startingWeightN, startingWeightD, err := con.Weight(ctx, baseTS, pSt)
	require.NoError(err)
	require.Equal(uint64(1), startingWeightD)

	wFun := func(ts consensus.TipSet) (uint64, uint64, error) {
		// No power-altering messages processed from here on out.
		// And so bootstrapSt correctly retrives power table for all
		// test blocks.
		return con.Weight(ctx, ts, bootstrapSt)
	}
	f1b1 := RequireMkFakeChildCore(require, baseTS, weightDeepGenCid, bootstrapStateRoot, uint64(0), uint64(0), wFun)
	f1b1.Miner = addr1
	f2b1 := RequireMkFakeChildCore(require, baseTS, weightDeepGenCid, bootstrapStateRoot, uint64(1), uint64(0), wFun)
	f2b1.Miner = addr2
	tsShared := consensus.RequireNewTipSet(require, f1b1, f2b1)

	// Sync first tipset, should have weight 22 + starting
	sharedCids := requirePutBlocks(require, cst, f1b1, f2b1)
	err = syncer.HandleNewBlocks(ctx, sharedCids)
	require.NoError(err)
	assertHead(assert, chain, tsShared)
	measuredWeight, denom, err := wFun(chain.Head())
	require.NoError(err)
	require.Equal(uint64(1), denom)
	expectedWeight := startingWeightN + uint64(22)
	assert.Equal(expectedWeight, measuredWeight)

	// fork 1 is heavier than the old head.
	f1b2a := RequireMkFakeChildCore(require, consensus.RequireNewTipSet(require, f1b1), weightDeepGenCid, bootstrapStateRoot, uint64(0), uint64(0), wFun)
	f1b2a.Miner = addr1
	f1b2b := RequireMkFakeChildCore(require, consensus.RequireNewTipSet(require, f1b1), weightDeepGenCid, bootstrapStateRoot, uint64(1), uint64(0), wFun)
	f1b2b.Miner = addr2
	f1 := consensus.RequireNewTipSet(require, f1b2a, f1b2b)
	f1Cids := requirePutBlocks(require, cst, f1.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, f1Cids)
	require.NoError(err)
	assertHead(assert, chain, f1)
	measuredWeight, denom, err = wFun(chain.Head())
	require.NoError(err)
	require.Equal(uint64(1), denom)
	expectedWeight = startingWeightN + uint64(33)
	assert.Equal(expectedWeight, measuredWeight)

	// fork 2 has heavier weight because of addr3's power even though there
	// are fewer blocks in the tipset than fork 1.
	f2b2 := RequireMkFakeChildCore(require, consensus.RequireNewTipSet(require, f2b1), weightDeepGenCid, bootstrapStateRoot, uint64(0), uint64(0), wFun)
	f2b2.Miner = addr3
	f2 := consensus.RequireNewTipSet(require, f2b2)
	f2Cids := requirePutBlocks(require, cst, f2.ToSlice()...)
	err = syncer.HandleNewBlocks(ctx, f2Cids)
	require.NoError(err)
	assertHead(assert, chain, f2)
	measuredWeight, denom, err = wFun(chain.Head())
	require.NoError(err)
	require.Equal(uint64(1), denom)
	expectedWeight = startingWeightN + uint64(119)
	assert.Equal(expectedWeight, measuredWeight)
}
