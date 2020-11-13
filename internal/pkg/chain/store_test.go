package chain_test

import (
	"context"
	ds "github.com/ipfs/go-datastore"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/cborutil"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

type CborBlockStore struct {
	*chain.Store
	cborStore cbor.IpldStore
}

func (cbor *CborBlockStore) PutBlocks(ctx context.Context, block []*block.Block) {
	_, _ = cbor.cborStore.Put(ctx, block)
}

// Default Chain diagram below.  Note that blocks in the same tipset are in parentheses.
//
// genesis -> (link1blk1, link1blk2) -> (link2blk1, link2blk2, link2blk3) -> link3blk1 -> (null block) -> (null block) -> (link4blk1, link4blk2)

// newChainStore creates a new chain store for tests.
/*func newChainStore(r repo.Repo, genCid cid.Cid) *CborBlockStore {
	dsstore := ds.NewMapDatastore()
	tempBlock := bstore.NewBlockstore(dsstore)
	cborStore := cbor.NewCborStore(tempBlock)
	//return chain.NewStore(r.Datastore(), cborStore, tempBlock, chain.NewStatusReporter(), block.UndefTipSet.Key(), genCid)
}*/
func newChainStore(r repo.Repo, genCid cid.Cid) *CborBlockStore {
	dsstore := ds.NewMapDatastore()
	tempBlock := bstore.NewBlockstore(dsstore)
	cborStore := cborutil.NewIpldStore(tempBlock)

	return &CborBlockStore{
		Store:     chain.NewStore(r.Datastore(), cborStore, tempBlock, chain.NewStatusReporter(), block.UndefTipSet.Key(), genCid),
		cborStore: cborStore,
	}
}

// requirePutTestChain puts the count tipsets preceding head in the source to
// the input chain store.
func requirePutTestChain(ctx context.Context, t *testing.T, cborStore *CborBlockStore, head block.TipSetKey, source *chain.Builder, count int) {
	tss := source.RequireTipSets(head, count)
	for _, ts := range tss {
		tsas := &chain.TipSetMetadata{
			TipSet:          ts,
			TipSetStateRoot: ts.At(0).StateRoot.Cid,
			TipSetReceipts:  types.EmptyReceiptsCID,
		}
		requirePutBlocksToCborStore(t, cborStore.cborStore, tsas.TipSet.Blocks()...)
		require.NoError(t, cborStore.PutTipSetMetadata(ctx, tsas))
	}
}

func requireGetTsasByParentAndHeight(t *testing.T, cborStore *CborBlockStore, pKey block.TipSetKey, h abi.ChainEpoch) []*chain.TipSetMetadata {
	tsasSlice, err := cborStore.GetTipSetAndStatesByParentsAndHeight(pKey, h)
	require.NoError(t, err)
	return tsasSlice
}

type HeadAndTipsetGetter interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
}

func requirePutBlocksToCborStore(t *testing.T, cst cbor.IpldStore, blocks ...*block.Block) {
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
	cs := newChainStore(r, genTS.At(0).Cid())

	genTsas := &chain.TipSetMetadata{
		TipSet:          genTS,
		TipSetStateRoot: genTS.At(0).StateRoot.Cid,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}
	err := cs.PutTipSetMetadata(ctx, genTsas)
	assert.NoError(t, err)
}

// Tipsets can be retrieved by key (all block cids).
func TestGetByKey(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	// Check that we can get all tipsets by key
	gotGTS := requireGetTipSet(ctx, t, cs, genTS.Key())
	gotGTSSR := requireGetTipSetStateRoot(ctx, t, cs, genTS.Key())

	got1TS := requireGetTipSet(ctx, t, cs, link1.Key())
	got1TSSR := requireGetTipSetStateRoot(ctx, t, cs, link1.Key())

	got2TS := requireGetTipSet(ctx, t, cs, link2.Key())
	got2TSSR := requireGetTipSetStateRoot(ctx, t, cs, link2.Key())

	got3TS := requireGetTipSet(ctx, t, cs, link3.Key())
	got3TSSR := requireGetTipSetStateRoot(ctx, t, cs, link3.Key())

	got4TS := requireGetTipSet(ctx, t, cs, link4.Key())
	got4TSSR := requireGetTipSetStateRoot(ctx, t, cs, link4.Key())
	assert.ObjectsAreEqualValues(genTS, gotGTS)
	assert.ObjectsAreEqualValues(link1, got1TS)
	assert.ObjectsAreEqualValues(link2, got2TS)
	assert.ObjectsAreEqualValues(link3, got3TS)
	assert.ObjectsAreEqualValues(link4, got4TS)

	assert.Equal(t, genTS.At(0).StateRoot.Cid, gotGTSSR)
	assert.Equal(t, link1.At(0).StateRoot.Cid, got1TSSR)
	assert.Equal(t, link2.At(0).StateRoot.Cid, got2TSSR)
	assert.Equal(t, link3.At(0).StateRoot.Cid, got3TSSR)
	assert.Equal(t, link4.At(0).StateRoot.Cid, got4TSSR)
}

// Tipsets can be retrieved by parent key (all block cids of parents).
func TestGetByParent(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	gotG := requireGetTsasByParentAndHeight(t, cs, block.TipSetKey{}, 0)
	got1 := requireGetTsasByParentAndHeight(t, cs, genTS.Key(), 1)
	got2 := requireGetTsasByParentAndHeight(t, cs, link1.Key(), 2)
	got3 := requireGetTsasByParentAndHeight(t, cs, link2.Key(), 3)
	got4 := requireGetTsasByParentAndHeight(t, cs, link3.Key(), 6) // two null blocks in between 3 and 4!

	assert.ObjectsAreEqualValues(genTS, gotG[0].TipSet)
	assert.ObjectsAreEqualValues(link1, got1[0].TipSet)
	assert.ObjectsAreEqualValues(link2, got2[0].TipSet)
	assert.ObjectsAreEqualValues(link3, got3[0].TipSet)
	assert.ObjectsAreEqualValues(link4, got4[0].TipSet)

	assert.Equal(t, genTS.At(0).StateRoot.Cid, gotG[0].TipSetStateRoot)
	assert.Equal(t, link1.At(0).StateRoot.Cid, got1[0].TipSetStateRoot)
	assert.Equal(t, link2.At(0).StateRoot.Cid, got2[0].TipSetStateRoot)
	assert.Equal(t, link3.At(0).StateRoot.Cid, got3[0].TipSetStateRoot)
	assert.Equal(t, link4.At(0).StateRoot.Cid, got4[0].TipSetStateRoot)
}

func TestGetMultipleByParent(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Put the test chain to the store
	requirePutTestChain(ctx, t, cs, link4.Key(), builder, 5)

	// Add extra children to the genesis tipset
	otherLink1 := builder.AppendOn(genTS, 1)
	otherRoot1 := types.CidFromString(t, "otherState")
	newChildTsas := &chain.TipSetMetadata{
		TipSet:          otherLink1,
		TipSetStateRoot: otherRoot1,
		TipSetReceipts:  types.EmptyReceiptsCID,
	}
	require.NoError(t, cs.PutTipSetMetadata(ctx, newChildTsas))
	gotNew1 := requireGetTsasByParentAndHeight(t, cs, genTS.Key(), 1)
	require.Equal(t, 2, len(gotNew1))
	for _, tsas := range gotNew1 {
		if tsas.TipSet.Len() == 1 {
			assert.ObjectsAreEqualValues(otherRoot1, tsas.TipSetStateRoot)
		} else {
			assert.ObjectsAreEqualValues(link1.At(0).StateRoot, tsas.TipSetStateRoot)
		}
	}
}

/* Head and its state is set and notified properly. */

// The constructor call sets the genesis cid for the chain store.
func TestSetGenesis(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	cs := newChainStore(r, genTS.At(0).Cid())

	require.Equal(t, genTS.At(0).Cid(), cs.GenesisCid())
}

func assertSetHead(t *testing.T, cborStore *CborBlockStore, ts *block.TipSet) {
	ctx := context.Background()
	err := cborStore.SetHead(ctx, ts)
	assert.NoError(t, err)
}

// Set and Get Head.
func TestHead(t *testing.T) {
	tf.UnitTest(t)

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	sr := chain.NewStatusReporter()
	bs := bstore.NewBlockstore(r.Datastore())
	cborStore := cbor.NewCborStore(bs)
	cs := chain.NewStore(r.Datastore(), cborStore, bs, sr, block.UndefTipSet.Key(), genTS.At(0).Cid())
	cboreStore := &CborBlockStore{
		Store: chain.NewStore(r.Datastore(), cborStore, bs, sr, block.UndefTipSet.Key(), genTS.At(0).Cid()),
	}
	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Head starts as an empty cid set
	assert.Equal(t, block.TipSetKey{}, cs.GetHead())

	// Set Head
	assertSetHead(t, cboreStore, genTS)
	assert.ObjectsAreEqualValues(genTS.Key(), cs.GetHead())
	assert.Equal(t, genTS.Key(), sr.Status().ValidatedHead)

	// Move head forward
	assertSetHead(t, cboreStore, link4)
	assert.ObjectsAreEqualValues(link4.Key(), cs.GetHead())
	assert.Equal(t, link4.Key(), sr.Status().ValidatedHead)

	// Move head back
	assertSetHead(t, cboreStore, link1)
	assert.ObjectsAreEqualValues(link1.Key(), cs.GetHead())
	assert.Equal(t, link1.Key(), sr.Status().ValidatedHead)
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

	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	r := repo.NewInMemoryRepo()
	chainStore := newChainStore(r, genTS.At(0).Cid())

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })
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
	heads := []*block.TipSet{genTS, link1, link2, link3, link4, link3, link2, link1, genTS}

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

	ctx := context.Background()
	builder := chain.NewBuilder(t, address.Undef)
	genTS := builder.Genesis()
	rPriv := repo.NewInMemoryRepo()
	ds := rPriv.Datastore()
	bs := bstore.NewBlockstore(ds)
	cst := cborutil.NewIpldStore(bs)

	// Construct test chain data
	link1 := builder.AppendOn(genTS, 2)
	link2 := builder.AppendOn(link1, 3)
	link3 := builder.AppendOn(link2, 1)
	link4 := builder.BuildOn(link3, 2, func(bb *chain.BlockBuilder, i int) { bb.IncHeight(2) })

	// Add blocks to blockstore
	requirePutBlocksToCborStore(t, cst, genTS.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link1.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link2.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link3.ToSlice()...)
	requirePutBlocksToCborStore(t, cst, link4.ToSlice()...)

	cboreStore := &CborBlockStore{
		Store:     chain.NewStore(ds, cst, bs, chain.NewStatusReporter(), block.UndefTipSet.Key(), genTS.At(0).Cid()),
		cborStore: cst,
	}
	requirePutTestChain(ctx, t, cboreStore, link4.Key(), builder, 5)
	assertSetHead(t, cboreStore, genTS) // set the genesis block

	assertSetHead(t, cboreStore, link4)
	cboreStore.Stop()

	// rebuild chain with same datastore and cborstore
	sr := chain.NewStatusReporter()
	rebootChain := chain.NewStore(ds, cst, bs, sr, block.UndefTipSet.Key(), genTS.At(0).Cid())
	rebootCbore := &CborBlockStore{
		Store: rebootChain,
	}

	err := rebootChain.Load(ctx)
	assert.NoError(t, err)
	assert.Equal(t, link4.Key(), sr.Status().ValidatedHead)

	// Check that chain store has index
	// Get a tipset and state by key
	got2 := requireGetTipSet(ctx, t, rebootCbore, link2.Key())
	assert.ObjectsAreEqualValues(link2, got2)

	// Get another by parent key
	got4 := requireGetTsasByParentAndHeight(t, rebootCbore, link3.Key(), 6)
	assert.Equal(t, 1, len(got4))
	assert.ObjectsAreEqualValues(link4, got4[0].TipSet)

	// Check the head
	assert.Equal(t, link4.Key(), rebootChain.GetHead())
}

func requireGetTipSet(ctx context.Context, t *testing.T, chainStore *CborBlockStore, key block.TipSetKey) *block.TipSet {
	ts, err := chainStore.GetTipSet(key)
	require.NoError(t, err)
	return ts
}

type tipSetStateRootGetter interface {
	GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error)
}

func requireGetTipSetStateRoot(ctx context.Context, t *testing.T, chainStore tipSetStateRootGetter, key block.TipSetKey) cid.Cid {
	stateCid, err := chainStore.GetTipSetStateRoot(key)
	require.NoError(t, err)
	return stateCid
}
