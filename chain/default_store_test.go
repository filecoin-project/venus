package chain

import (
	"context"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

// Note: many of these tests use the test chain defined in the init function of default_syncer_test.
func initStoreTest(ctx context.Context, require *require.Assertions) {
	powerTable := &testhelpers.TestView{}
	r := repo.NewInMemoryRepo()
	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.NewCborStore()
	con := consensus.NewExpected(cst, bs, testhelpers.NewTestProcessor(), powerTable, genCid, proofs.NewFakeProver(true, nil))
	initSyncTest(require, con, consensus.InitGenesis, cst, bs, r)
	requireSetTestChain(require, con, true)
}

func newChainStore() Store {
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	return NewDefaultStore(ds, hamt.NewCborStore(), genCid)
}

// RequirePutTestChain adds all test chain tipsets to the passed in chain store.
func RequirePutTestChain(require *require.Assertions, chain Store) {
	ctx := context.Background()
	genTsas := &TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genStateRoot,
	}
	link1Tsas := &TipSetAndState{
		TipSet:          link1,
		TipSetStateRoot: link1State,
	}
	link2Tsas := &TipSetAndState{
		TipSet:          link2,
		TipSetStateRoot: link2State,
	}
	link3Tsas := &TipSetAndState{
		TipSet:          link3,
		TipSetStateRoot: link3State,
	}
	link4Tsas := &TipSetAndState{
		TipSet:          link4,
		TipSetStateRoot: link4State,
	}
	RequirePutTsas(ctx, require, chain, genTsas)
	RequirePutTsas(ctx, require, chain, link1Tsas)
	RequirePutTsas(ctx, require, chain, link2Tsas)
	RequirePutTsas(ctx, require, chain, link3Tsas)
	RequirePutTsas(ctx, require, chain, link4Tsas)
}

func requireGetTsas(ctx context.Context, require *require.Assertions, chain Store, key string) *TipSetAndState {
	tsas, err := chain.GetTipSetAndState(ctx, key)
	require.NoError(err)
	return tsas
}

func requireGetTsasByParentAndHeight(ctx context.Context, require *require.Assertions, chain Store, pKey string, h uint64) []*TipSetAndState {
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
	chain := newChainStore()
	genTsas := &TipSetAndState{
		TipSet:          genTS,
		TipSetStateRoot: genStateRoot,
	}
	err := chain.PutTipSetAndState(ctx, genTsas)
	assert.NoError(err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()

	RequirePutTestChain(require, chain)
	kg := genTS.String()
	k1 := link1.String()
	k2 := link2.String()
	k3 := link3.String()
	k4 := link4.String()

	gotG := requireGetTsas(ctx, require, chain, kg)
	got1 := requireGetTsas(ctx, require, chain, k1)
	got2 := requireGetTsas(ctx, require, chain, k2)
	got3 := requireGetTsas(ctx, require, chain, k3)
	got4 := requireGetTsas(ctx, require, chain, k4)

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

	RequirePutTestChain(require, chain)
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
	chain := newChainStore()

	RequirePutTestChain(require, chain)
	pk1 := genTS.String()
	// give one parent multiple children and then query
	newBlk := RequireMkFakeChild(require,
		FakeChildParams{Parent: genTS, GenesisCid: genCid, StateRoot: genStateRoot, Nonce: uint64(5)})
	newChild := testhelpers.RequireNewTipSet(require, newBlk)
	newRoot := cidGetter()
	newChildTsas := &TipSetAndState{
		TipSet:          newChild,
		TipSetStateRoot: newRoot,
	}
	RequirePutTsas(ctx, require, chain, newChildTsas)
	gotNew1 := requireGetTsasByParentAndHeight(ctx, require, chain, pk1, uint64(1))
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

	RequirePutTestChain(require, chain)

	var cids types.SortedCidSet
	for _, blk := range blks {
		c := blk.Cid()
		(&cids).Add(c)
		gotBlk, err := chain.GetBlock(ctx, c)
		assert.NoError(err)
		assert.Equal(blk, gotBlk)
	}
	gotBlks, err := chain.GetBlocks(ctx, cids)
	assert.NoError(err)
	assert.Equal(len(blks), len(gotBlks))
}

// Store correctly indicates that is has all blocks in put tipsets
func TestHasAllBlocks(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()

	blks := []*types.Block{genesis, link1blk1, link1blk2, link2blk1,
		link2blk2, link2blk3, link3blk1, link4blk1, link4blk2}
	assert.False(chain.HasBlock(ctx, genCid)) // Has returns false before put

	RequirePutTestChain(require, chain)

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
	RequirePutTestChain(require, chain)
	require.Equal(genCid, chain.GenesisCid())
}

func assertSetHead(assert *assert.Assertions, chain Store, ts consensus.TipSet) {
	ctx := context.Background()
	err := chain.SetHead(ctx, ts)
	assert.NoError(err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	chain := newChainStore()
	RequirePutTestChain(require, chain)

	// Head starts as nil
	assert.Nil(chain.Head())

	// Set Head
	assertSetHead(assert, chain, genTS)
	assert.Equal(genTS, chain.Head())

	// Move head forward
	assertSetHead(assert, chain, link4)
	assert.Equal(link4, chain.Head())

	// Move head back
	assertSetHead(assert, chain, genTS)
	assert.Equal(genTS, chain.Head())
}

// LatestState correctly returns the state of the head.
func TestLatestState(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	require := require.New(t)
	assert := assert.New(t)
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()
	bs := bstore.NewBlockstore(ds)
	cst := hamt.NewCborStore()
	chain := NewDefaultStore(ds, cst, genCid)

	RequirePutTestChain(require, chain)

	// LatestState errors without a set head
	_, err := chain.LatestState(ctx)
	assert.Error(err)

	// Call init genesis again to load genesis state into cbor store.
	// This is required for the chain to access the state in the cbor store.
	_, err = consensus.InitGenesis(cst, bs)
	require.NoError(err)

	assertSetHead(assert, chain, genTS)
	st, err := chain.LatestState(ctx)
	require.NoError(err)
	c, err := st.Flush(ctx)
	require.NoError(err)
	assert.Equal(genStateRoot, c)
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
	chain := newChainStore()
	RequirePutTestChain(require, chain)

	ps := chain.HeadEvents()
	chA := ps.Sub(NewHeadTopic)
	chB := ps.Sub(NewHeadTopic)

	assertSetHead(assert, chain, genTS)
	assertSetHead(assert, chain, link1)
	assertSetHead(assert, chain, link2)
	assertSetHead(assert, chain, link3)
	assertSetHead(assert, chain, link4)
	assertSetHead(assert, chain, link3)
	assertSetHead(assert, chain, link2)
	assertSetHead(assert, chain, link1)
	assertSetHead(assert, chain, genTS)
	heads := []consensus.TipSet{genTS, link1, link2, link3, link4, link3,
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

/* Block history */

// Block history reports all ancestors in the chain
func TestBlockHistory(t *testing.T) {
	ctx := context.Background()
	initStoreTest(ctx, require.New(t))
	assert := assert.New(t)
	require := require.New(t)
	chain := newChainStore()
	RequirePutTestChain(require, chain)
	assertSetHead(assert, chain, genTS) // set the genesis block

	assertSetHead(assert, chain, link4)
	historyCh := chain.BlockHistory(ctx)

	assert.Equal(link4, ((<-historyCh).(consensus.TipSet)))
	assert.Equal(link3, ((<-historyCh).(consensus.TipSet)))
	assert.Equal(link2, ((<-historyCh).(consensus.TipSet)))
	assert.Equal(link1, ((<-historyCh).(consensus.TipSet)))
	assert.Equal(genTS, ((<-historyCh).(consensus.TipSet)))

	ts, more := <-historyCh
	assert.Equal(nil, ts)     // Genesis block has no parent.
	assert.Equal(false, more) // Channel is closed
}

func TestBlockHistoryCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	initStoreTest(ctx, require.New(t))
	assert := assert.New(t)
	require := require.New(t)
	chain := newChainStore()
	RequirePutTestChain(require, chain)
	assertSetHead(assert, chain, genTS) // set the genesis block

	assertSetHead(assert, chain, link4)
	historyCh := chain.BlockHistory(ctx)

	assert.Equal(link4, ((<-historyCh).(consensus.TipSet)))
	assert.Equal(link3, ((<-historyCh).(consensus.TipSet)))
	cancel()
	time.Sleep(10 * time.Millisecond)

	ts, more := <-historyCh
	// Channel is closed
	assert.Equal(nil, ts)
	assert.Equal(false, more)
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
	chain := NewDefaultStore(ds, hamt.NewCborStore(), genCid)
	RequirePutTestChain(require, chain)
	assertSetHead(assert, chain, genTS) // set the genesis block

	assertSetHead(assert, chain, link4)
	chain.Stop()

	// rebuild chain with same datastore
	rebootChain := NewDefaultStore(ds, hamt.NewCborStore(), genCid)
	err := rebootChain.Load(ctx)
	assert.NoError(err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTsas(ctx, require, rebootChain, link2.String())
	assert.Equal(link2, got2.TipSet)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(ctx, require, rebootChain, link3.String(), uint64(6))
	assert.Equal(1, len(got4))
	assert.Equal(link4, got4[0].TipSet)

	// Check that chain store has blocks
	assert.True(rebootChain.HasBlock(ctx, link3blk1.Cid()))
	assert.True(rebootChain.HasBlock(ctx, link2blk3.Cid()))
	assert.True(rebootChain.HasBlock(ctx, genesis.Cid()))
}
