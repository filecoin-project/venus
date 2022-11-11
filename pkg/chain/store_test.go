// stm: #unit
package chain_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/testhelpers"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/pkg/util/test"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type CborBlockStore struct {
	*chain.Store
	cborStore cbor.IpldStore
}

func (cbor *CborBlockStore) PutBlocks(ctx context.Context, blocks []*types.BlockHeader) {
	for _, blk := range blocks {
		_, _ = cbor.cborStore.Put(ctx, blk)
	}
}

// Default Chain diagram below.  Note that blocks in the same tipset are in parentheses.
//
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (null block) -> (null block) -> (link4blk1, link4blk2)

// newChainStore creates a new chain store for tests.
func newChainStore(r repo.Repo, genTS *types.TipSet) *CborBlockStore {
	tempBlock := r.Datastore()
	cborStore := cbor.NewCborStore(tempBlock)
	return &CborBlockStore{
		Store:     chain.NewStore(r.ChainDatastore(), tempBlock, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator()),
		cborStore: cborStore,
	}
}

// requirePutTestChain puts the count tipsets preceding head in the source to
// the input chain store.
func requirePutTestChain(ctx context.Context, t *testing.T, cborStore *CborBlockStore, head types.TipSetKey, source *chain.Builder, count int) {
	tss := source.RequireTipSets(ctx, head, count)
	for _, ts := range tss {
		tsas := &chain.TipSetMetadata{
			TipSet:          ts,
			TipSetStateRoot: ts.At(0).ParentStateRoot,
			TipSetReceipts:  testhelpers.EmptyReceiptsCID,
		}
		requirePutBlocksToCborStore(t, cborStore.cborStore, tsas.TipSet.Blocks()...)
		require.NoError(t, cborStore.Store.PutTipSetMetadata(ctx, tsas))
	}
}

type HeadAndTipsetGetter interface {
	GetHead() types.TipSetKey
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

func requirePutBlocksToCborStore(t *testing.T, cst cbor.IpldStore, blocks ...*types.BlockHeader) {
	for _, block := range blocks {
		_, err := cst.Put(context.Background(), block)
		require.NoError(t, err)
	}
}

/* Putting and getting tipsets and states. */

// Adding tipsets to the store doesn't error.
func TestPutTipSet(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	genTsas := &chain.TipSetMetadata{
		TipSet:          genTS,
		TipSetStateRoot: genTS.At(0).ParentStateRoot,
		TipSetReceipts:  testhelpers.EmptyReceiptsCID,
	}
	err := cs.Store.PutTipSetMetadata(ctx, genTsas)
	assert.NoError(t, err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	// Construct test chain data
	link1 := builder.AppendOn(ctx, genTS, 2)
	link2 := builder.AppendOn(ctx, link1, 3)
	link3 := builder.AppendOn(ctx, link2, 1)
	link4 := builder.BuildOn(ctx, link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	// Check that we can get all tipsets by key
	gotGTS := requireGetTipSet(ctx, t, cs, genTS.Key())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, cs, genTS)

	got1TS := requireGetTipSet(ctx, t, cs, link1.Key())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, cs, link1)

	got2TS := requireGetTipSet(ctx, t, cs, link2.Key())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, cs, link2)

	got3TS := requireGetTipSet(ctx, t, cs, link3.Key())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, cs, link3)

	got4TS := requireGetTipSet(ctx, t, cs, link4.Key())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, cs, link4)
	assert.ObjectsAreEqualValues(genTS, gotGTS)
	assert.ObjectsAreEqualValues(link1, got1TS)
	assert.ObjectsAreEqualValues(link2, got2TS)
	assert.ObjectsAreEqualValues(link3, got3TS)
	assert.ObjectsAreEqualValues(link4, got4TS)

	assert.Equal(t, genTS.At(0).ParentStateRoot, gotGTSSR)
	assert.Equal(t, link1.At(0).ParentStateRoot, got1TSSR)
	assert.Equal(t, link2.At(0).ParentStateRoot, got2TSSR)
	assert.Equal(t, link3.At(0).ParentStateRoot, got3TSSR)
	assert.Equal(t, link4.At(0).ParentStateRoot, got4TSSR)
}

// Tipsets can be retrieved by key (all block cids).
func TestRevertChange(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.TODO()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	cs := newChainStore(builder.Repo(), genTS)
	genesis := builder.Genesis()

	link1 := builder.AppendOn(ctx, genesis, 1)
	link2 := builder.AppendOn(ctx, link1, 1)
	link3 := builder.AppendOn(ctx, link2, 1)

	err := cs.Store.SetHead(ctx, link3)
	require.NoError(t, err)

	link4 := builder.AppendOn(ctx, genesis, 2)
	link5 := builder.AppendOn(ctx, link4, 2)
	link6 := builder.AppendOn(ctx, link5, 2)

	ch := cs.Store.SubHeadChanges(ctx)
	currentA := <-ch
	test.Equal(t, currentA[0].Type, types.HCCurrent)
	test.Equal(t, currentA[0].Val, link3)

	err = cs.Store.SetHead(ctx, link6)
	require.NoError(t, err)
	headChanges := <-ch

	if len(headChanges) == 1 {
		// maybe link3, if link3 fetch next
		headChanges = <-ch
	}
	test.Equal(t, headChanges[0].Type, types.HCRevert)
	test.Equal(t, headChanges[0].Val, link3)
	test.Equal(t, headChanges[1].Type, types.HCRevert)
	test.Equal(t, headChanges[1].Val, link2)
	test.Equal(t, headChanges[2].Type, types.HCRevert)
	test.Equal(t, headChanges[2].Val, link1)

	test.Equal(t, headChanges[3].Type, types.HCApply)
	test.Equal(t, headChanges[3].Val, link4)
	test.Equal(t, headChanges[4].Type, types.HCApply)
	test.Equal(t, headChanges[4].Val, link5)
	test.Equal(t, headChanges[5].Type, types.HCApply)
	test.Equal(t, headChanges[5].Val, link6)
}

/* Head and its state is set and notified properly. */

// The constructor call sets the genesis cid for the chain store.
func TestSetGenesis(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS)

	require.Equal(t, genTS.At(0).Cid(), cs.Store.GenesisCid())
}

func assertSetHead(t *testing.T, cborStore *CborBlockStore, ts *types.TipSet) {
	ctx := context.Background()
	err := cborStore.Store.SetHead(ctx, ts)
	assert.NoError(t, err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.TODO()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := builder.Repo()
	bs := builder.BlockStore()
	cs := chain.NewStore(r.ChainDatastore(), bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator())
	cboreStore := &CborBlockStore{
		Store: chain.NewStore(r.ChainDatastore(), bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator()),
	}
	// Construct test chain data
	link1 := builder.AppendOn(ctx, genTS, 2)
	link2 := builder.AppendOn(ctx, link1, 3)
	link3 := builder.AppendOn(ctx, link2, 1)
	link4 := builder.BuildOn(ctx, link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Head starts as an empty cid set
	assert.Equal(t, types.UndefTipSet, cs.GetHead())

	// Set Head
	assertSetHead(t, cboreStore, genTS)
	assert.ObjectsAreEqualValues(genTS.Key(), cs.GetHead())

	// Move head forward
	assertSetHead(t, cboreStore, link4)
	assert.ObjectsAreEqualValues(link4.Key(), cs.GetHead())

	// Move head back
	assertSetHead(t, cboreStore, link1)
	assert.ObjectsAreEqualValues(link1.Key(), cs.GetHead())
}

func assertEmptyCh(t *testing.T, ch <-chan []*types.HeadChange) {
	select {
	case <-ch:
		assert.True(t, false)
	default:
	}
}

// Head events are propagated on HeadEvents.
func TestHeadEvents(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	chainStore := newChainStore(builder.Repo(), genTS)
	// Construct test chain data
	link1 := builder.AppendOn(ctx, genTS, 2)
	link2 := builder.AppendOn(ctx, link1, 3)
	link3 := builder.AppendOn(ctx, link2, 1)
	link4 := builder.BuildOn(ctx, link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })
	chA := chainStore.Store.SubHeadChanges(ctx)
	chB := chainStore.Store.SubHeadChanges(ctx)
	// HCurrent
	<-chA
	<-chB

	defer ctx.Done()

	headSets := []*types.TipSet{genTS, link1, link2, link3, link4, link3, link2, link1, genTS}
	heads := []*types.TipSet{genTS, link1, link2, link3, link4, link4, link3, link2, link1}
	types := []types.HeadChangeType{
		types.HCApply, types.HCApply, types.HCApply, types.HCApply, types.HCApply, types.HCRevert,
		types.HCRevert, types.HCRevert, types.HCRevert,
	}
	waitAndCheck := func(index int) {
		headA := <-chA
		headB := <-chB
		assert.Equal(t, headA[0].Type, types[index])
		test.Equal(t, headA, headB)
		test.Equal(t, headA[0].Val, heads[index])
	}

	// Heads arrive in the expected order
	for i := 0; i < 9; i++ {
		assertSetHead(t, chainStore, headSets[i])
		waitAndCheck(i)
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

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	rPriv := repo.NewInMemoryRepo()
	bs := rPriv.Datastore()
	ds := rPriv.ChainDatastore()
	cst := cbor.NewCborStore(bs)

	// Construct test chain data
	link1 := builder.AppendOn(ctx, genTS, 2)
	link2 := builder.AppendOn(ctx, link1, 3)
	link3 := builder.AppendOn(ctx, link2, 1)
	link4 := builder.BuildOn(ctx, link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Add blocks to blockstore
	requirePutBlocksToCborStore(t, cst, genTS.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link1.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link2.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link3.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link4.ToSlice()...)

	cborStore := &CborBlockStore{
		Store:     chain.NewStore(ds, bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator()),
		cborStore: cst,
	}
	requirePutTestChain(ctx, t, cborStore, link4.Key(), builder, 5)
	assertSetHead(t, cborStore, genTS) // set the genesis block

	assertSetHead(t, cborStore, link4)
	cborStore.Store.Stop()

	// rebuild chain with same datastore and cborstore
	rebootChain := chain.NewStore(ds, bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator())
	rebootCbore := &CborBlockStore{
		Store: rebootChain,
	}

	// stm: @CHAIN_STORE_LOAD_001
	err := rebootChain.Load(ctx)
	assert.NoError(t, err)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootCbore, link2.Key())
	assert.ObjectsAreEqualValues(link2, got2)

	// Check the head
	test.Equal(t, link4, rebootChain.GetHead())

	{
		assert.NoError(t, rebootChain.Blockstore().DeleteBlock(ctx, link3.Blocks()[0].Cid()))
		newStore := chain.NewStore(ds, bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator())
		// error occurs while getting tipset identified by parent's cid block,
		//  because block[0] has been deleted.
		// stm: @CHAIN_STORE_LOAD_003
		assert.Error(t, newStore.Load(ctx))
	}

	{
		assert.NoError(t, ds.Put(ctx, chain.HeadKey, []byte("bad chain head data")))
		newStore := chain.NewStore(ds, bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator())
		// error occurs while getting tipset identified by parent's cid block
		// stm: @CHAIN_STORE_LOAD_002
		assert.Error(t, newStore.Load(ctx))
	}
}

func TestLoadTipsetMeta(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	rPriv := repo.NewInMemoryRepo()
	bs := rPriv.Datastore()
	ds := rPriv.ChainDatastore()
	cst := cbor.NewCborStore(bs)

	count := 30
	links := make([]*types.TipSet, count)
	links[0] = genTS

	for i := 1; i < count-1; i++ {
		links[i] = builder.AppendOn(ctx, links[i-1], rand.Intn(2)+1)
	}
	head := builder.BuildOn(ctx, links[count-2], 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })
	links[count-1] = head

	// Add blocks to blockstore
	for _, ts := range links {
		requirePutBlocksToCborStore(t, cst, ts.ToSlice()...)
	}

	chain.DefaultChainIndexCacheSize = 2
	chain.DefaultTipsetLruCacheSize = 2

	cs := chain.NewStore(ds, bs, genTS.At(0).Cid(), chain.NewMockCirculatingSupplyCalculator())
	cborStore := &CborBlockStore{Store: cs, cborStore: cst}

	requirePutTestChain(ctx, t, cborStore, head.Key(), builder, 5)
	assertSetHead(t, cborStore, head)

	// stm: @CHAIN_STORE_LOAD_METADATA_001
	meta, err := cs.LoadTipsetMetadata(ctx, head)
	assert.NoError(t, err)
	assert.NotNil(t, meta)

	flyingTipset := builder.BuildOrphaTipset(head, 2, nil)
	{ // Chain store load tipset meta
		// should not be found.
		// stm: @CHAIN_STORE_LOAD_METADATA_002
		_, err = cs.LoadTipsetMetadata(ctx, flyingTipset)
		assert.Error(t, err)
		// put invalid data for newTs.
		key := datastore.NewKey(fmt.Sprintf("p-%s h-%d", flyingTipset.String(), flyingTipset.Height()))
		assert.NoError(t, ds.Put(ctx, key, []byte("invalid tipset data")))
		// error getting object from store providing key,
		// stm: @CHAIN_STORE_LOAD_METADATA_002
		_, err = cs.LoadTipsetMetadata(ctx, flyingTipset)
		assert.Error(t, err)
		assert.NoError(t, ds.Delete(ctx, key))
	}
	{ // Chain store get blocks
		// stm: @CHAIN_STORE_GET_BLOCK_001
		block, err := cs.GetBlock(ctx, head.Key().Cids()[0])
		assert.NoError(t, err)
		assert.Equal(t, block.Cid(), head.Blocks()[0].Cid())
		// error getting block from ilpd storage
		// stm: @CHAIN_STORE_LOAD_002
		_, err = cs.GetBlock(ctx, flyingTipset.Cids()[0])
		assert.Error(t, err)
	}
	{ // Chain store get tipset
		// stm: @CHAIN_STORE_GET_TIPSET_001
		ts, err := cs.GetTipSet(ctx, head.Key())
		assert.NoError(t, err)
		assert.Equal(t, ts.Key(), head.Key())

		// If the key is empty, return current head tipset cids.
		// stm: @CHAIN_STORE_GET_TIPSET_002
		ts, err = cs.GetTipSet(ctx, types.EmptyTSK)
		assert.NoError(t, err)
		assert.Equal(t, ts.Key(), head.Key())

		// The head is cached now.
		// stm: @CHAIN_STORE_GET_TIPSET_003
		ts, err = cs.GetTipSet(ctx, head.Key())
		assert.NoError(t, err)
		assert.Equal(t, ts.Key(), head.Key())

		// error getting blocks
		// stm: @CHAIN_STORE_GET_TIPSET_004
		_, err = cs.GetTipSet(ctx, flyingTipset.Key())
		assert.Error(t, err)
	}
	{ // Chain store get tipset by height
		targetTS := links[abi.ChainEpoch(count/2)]
		// stm: @CHAIN_STORE_GET_TIPSET_BY_HEIGHT_001
		ts, err := cs.GetTipSetByHeight(ctx, head, targetTS.Height(), true)
		assert.NoError(t, err)
		assert.Equal(t, ts.Key(), targetTS.Key())

		// The epoch is greater than the tipset's height
		// stm: @CHAIN_STORE_GET_TIPSET_BY_HEIGHT_002
		_, err = cs.GetTipSetByHeight(ctx, head, head.Height()+1, true)
		assert.Error(t, err)

		// targetTs.Height - 1 would make sure tipset was not cached
		targetTS = links[targetTS.Height()-1]
		blockCid := targetTS.Cids()[0]
		block, err := bs.Get(ctx, blockCid)
		assert.NoError(t, err)
		assert.NoError(t, bs.DeleteBlock(ctx, targetTS.Cids()[0]))
		// error occurs retrieving the tipset from the chain index
		// stm: @CHAIN_STORE_GET_TIPSET_BY_HEIGHT_004
		_, err = cs.GetTipSetByHeight(ctx, head, targetTS.Height(), true)
		assert.Error(t, err)

		// restore deleted block.
		assert.NoError(t, bs.Put(ctx, block))
	}
	{ // Get tipset state
		parentHead := links[len(links)-2]
		// stm: @CHAIN_STORE_GET_TIPSET_STATE_ROOT_001
		stateRoot, err := cs.GetTipSetStateRoot(ctx, parentHead)
		assert.NoError(t, err)
		assert.Equal(t, head.ParentState(), stateRoot)

		// stm: @CHAIN_STORE_GET_TIPSET_STATE_ROOT_001
		_, err = cs.GetTipSetStateRoot(ctx, flyingTipset)
		assert.Error(t, err)

		// error occurs while trying to return tipsetStateRoot from tipIndex. not exist
		// stm: @CHAIN_STORE_GET_TIPSET_STATE_002
		_, err = cs.GetTipSetState(ctx, flyingTipset)
		assert.Error(t, err)
	}
	{ // Get genesis block
		genesisBlock, err := cs.GetGenesisBlock(ctx)
		assert.NoError(t, err)
		assert.Equal(t, genesisBlock.Cid(), genTS.Blocks()[0].Cid())
	}
	{ // Beacon entry
		ts := links[6]
		// stm: @CHAIN_STORE_GET_LATEST_BEACON_ENTRY_001
		entry, err := cs.GetLatestBeaconEntry(ctx, ts)
		assert.NoError(t, err)
		assert.Greater(t, len(entry.Data), 0)

		// no beacon entries is found in the 20 block prior to given tipset
		// stm: @CHAIN_STORE_GET_LATEST_BEACON_ENTRY_004
		_, err = cs.GetLatestBeaconEntry(ctx, head)
		assert.Error(t, err)

		deletedCid := ts.Parents().Cids()[0]
		block, err := bs.Get(ctx, deletedCid)
		assert.NoError(t, err)
		assert.NoError(t, bs.DeleteBlock(ctx, deletedCid))
		// loading parents failed.
		// stm: @CHAIN_STORE_GET_LATEST_BEACON_ENTRY_003
		_, err = cs.GetLatestBeaconEntry(ctx, ts)
		assert.Error(t, err)

		// recover deleted block
		assert.NoError(t, bs.Put(ctx, block))
	}
}

func requireGetTipSet(ctx context.Context, t *testing.T, chainStore *CborBlockStore, key types.TipSetKey) *types.TipSet {
	ts, err := chainStore.Store.GetTipSet(ctx, key)
	require.NoError(t, err)
	return ts
}

type tipSetStateRootGetter interface {
	GetTipSetStateRoot(context.Context, *types.TipSet) (cid.Cid, error)
}

func requireGetTipSetStateRoot(ctx context.Context, t *testing.T, chainStore tipSetStateRootGetter, ts *types.TipSet) cid.Cid {
	stateCid, err := chainStore.GetTipSetStateRoot(ctx, ts)
	require.NoError(t, err)
	return stateCid
}
