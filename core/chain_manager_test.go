package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	"gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testGenesis, block1, block2, fork1, fork2, fork3             *types.Block
	tipsetA1, tipsetA2, tipsetA3, tipsetB1, tipsetB2, tipsetFork *types.Block
	tipsetA, tipsetB                                             TipSet

	bad1, bad2, bad3 *types.Block
)

func init() {
	cst := hamt.NewCborStore()
	genesis, err := InitGenesis(cst)
	if err != nil {
		panic(err)
	}
	testGenesis = genesis

	block1 = MkChild([]*types.Block{testGenesis}, genesis.StateRoot, 0)
	block2 = MkChild([]*types.Block{block1}, block1.StateRoot, 0)

	fork1 = MkChild([]*types.Block{testGenesis}, genesis.StateRoot, 1)
	fork2 = MkChild([]*types.Block{fork1}, fork1.StateRoot, 1)
	fork3 = MkChild([]*types.Block{fork2}, fork2.StateRoot, 1)

	tipsetA1 = MkChild([]*types.Block{testGenesis}, genesis.StateRoot, 2)
	tipsetA2 = MkChild([]*types.Block{testGenesis}, genesis.StateRoot, 3)
	tipsetA3 = MkChild([]*types.Block{testGenesis}, genesis.StateRoot, 4)
	tipsetA = NewTipSet(tipsetA1, tipsetA2, tipsetA3)

	tipsetB1 = MkChild(tipsetA.ToSlice(), genesis.StateRoot, 0)
	tipsetB2 = MkChild(tipsetA.ToSlice(), genesis.StateRoot, 1)
	tipsetB = NewTipSet(tipsetB1, tipsetB2)

	tipsetFork = MkChild([]*types.Block{tipsetA1, tipsetA3}, genesis.StateRoot, 0)

	bad1 = &types.Block{
		StateRoot: testGenesis.StateRoot,
		Nonce:     404,
	}
	bad2 = MkChild([]*types.Block{bad1}, bad1.StateRoot, 0)

	bad3 = MkChild([]*types.Block{tipsetA1, block2}, genesis.StateRoot, 0)
}

func addBlocks(t *testing.T, cs *hamt.CborIpldStore, blks ...*types.Block) {
	for _, blk := range blks {
		_, err := cs.Put(context.Background(), blk)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func newTestUtils() (context.Context, *hamt.CborIpldStore, datastore.Datastore, *ChainManager) {
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)
	return ctx, cs, ds, stm
}

func requireProcessBlock(ctx context.Context, t *testing.T, cm *ChainManager, b *types.Block) {
	require := require.New(t)
	_, err := cm.ProcessNewBlock(ctx, b)
	require.NoError(err)
}

func TestBasicAddBlock(t *testing.T) {
	ctx, _, _, stm := newTestUtils()
	assert := assert.New(t)

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	res, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), block1.Cid())
	assert.True(stm.knownGoodBlocks.Has(block1.Cid()))
	res, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), block2.Cid())
	assert.True(stm.knownGoodBlocks.Has(block2.Cid()))
}

func TestMultiBlockTipsetAdd(t *testing.T) {
	ctx, _, _, stm := newTestUtils()
	assert := assert.New(t)

	// Add tipsetA
	assert.NoError(stm.Genesis(ctx, InitGenesis))
	res, err := stm.ProcessNewBlock(ctx, tipsetA1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	res, err = stm.ProcessNewBlock(ctx, tipsetA2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	res, err = stm.ProcessNewBlock(ctx, tipsetA3)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	assert.Equal(stm.GetHeaviestTipSet(), tipsetA)

	// Add tipsetB
	res, err = stm.ProcessNewBlock(ctx, tipsetB1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), tipsetB1.Cid())
	res, err = stm.ProcessNewBlock(ctx, tipsetB2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	assert.Equal(stm.GetHeaviestTipSet(), tipsetB)
}

func TestHeaviestTipSetPubSub(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()
	ch := stm.HeaviestTipSetPubSub.Sub(HeaviestTipSetTopic)

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	block3 := MkChild([]*types.Block{block2}, block2.StateRoot, 0)
	block4 := MkChild([]*types.Block{block2}, block2.StateRoot, 1)
	blocks := []*types.Block{block1, block2, block3, block4}
	expTipSets := []TipSet{
		NewTipSet(block1),
		NewTipSet(block2),
		NewTipSet(block3), // intermediate tipset is published
		NewTipSet(block3, block4),
	}
	for _, b := range blocks {
		stm.ProcessNewBlock(ctx, b)
	}
	gotTSs := map[string]bool{}
	for i := 0; i < 5; i++ {
		gotTipSet := <-ch
		gotTSs[gotTipSet.(TipSet).String()] = true
	}

	for _, ts := range expTipSets {
		assert.True(gotTSs[ts.String()])
	}
	assert.Equal(len(expTipSets), len(gotTSs)-1) // -1 accounts for genesis publish
}

func TestForkChoice(t *testing.T) {
	assert := assert.New(t)
	ctx, cs, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	res, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), block1.Cid())
	assert.True(stm.knownGoodBlocks.Has(block1.Cid()))

	// progress to block2 block on our chain
	res, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), block2.Cid())
	assert.True(stm.knownGoodBlocks.Has(block2.Cid()))

	// Now, introduce a valid fork
	addBlocks(t, cs, fork1, fork2)

	res, err = stm.ProcessNewBlock(ctx, fork3)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), fork3.Cid())

	cids, err := stm.readHeaviestTipSetCids()
	assert.NoError(err)
	assert.Equal(cids.Len(), 1)
	bbc := cids.Iter().Value()
	assert.Equal(fork3.Cid(), bbc)
}

func TestMultiBlockTipsetForkChoice(t *testing.T) {
	ctx, _, _, stm := newTestUtils()
	assert := assert.New(t)

	// Progress to tipsetB
	assert.NoError(stm.Genesis(ctx, InitGenesis))
	requireProcessBlock(ctx, t, stm, tipsetA1)
	requireProcessBlock(ctx, t, stm, tipsetA2)
	requireProcessBlock(ctx, t, stm, tipsetA3)
	requireProcessBlock(ctx, t, stm, tipsetB1)
	assert.Equal(stm.GetBestBlock().Cid(), tipsetB1.Cid())
	requireProcessBlock(ctx, t, stm, tipsetB2)
	assert.Equal(stm.GetHeaviestTipSet(), tipsetB)

	// Introduce a losing fork
	res, err := stm.ProcessNewBlock(ctx, tipsetFork)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(stm.GetHeaviestTipSet(), tipsetB)
	cids, err := stm.readHeaviestTipSetCids()
	assert.NoError(err)
	assert.True(cids.Equals(tipsetB.ToSortedCidSet()))
}

func TestRejectShorterChain(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	res, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), block1.Cid())

	res, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.GetBestBlock().Cid(), block2.Cid())

	// block with lower height than our current shouldnt fail, but it shouldnt be accepted as the best block
	res, err = stm.ProcessNewBlock(ctx, fork1)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(stm.GetBestBlock().Cid(), block2.Cid())

	// block with same height as our current should fail
	res, err = stm.ProcessNewBlock(ctx, fork2)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(stm.GetBestBlock().Cid(), block2.Cid())
}

func TestKnownAncestor(t *testing.T) {
	assert := assert.New(t)
	ctx, cs, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	addBlocks(t, cs, block1)
	res, err := stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	addBlocks(t, cs, fork1, fork2)
	base, chain, err := stm.findKnownAncestor(ctx, fork3)
	assert.NoError(err)
	assert.Equal(NewTipSet(testGenesis), base)
	assert.Len(chain, 3)
	assert.Equal(NewTipSet(fork3), chain[0])
	assert.Equal(NewTipSet(fork2), chain[1])
	assert.Equal(NewTipSet(fork1), chain[2])
}

func TestGenesis(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	assert.Equal(testGenesis, stm.heaviestTipSet.ts.ToSlice()[0])
	assert.True(stm.knownGoodBlocks.Has(testGenesis.Cid()))

	var i interface{}
	assert.NoError(stm.cstore.Get(ctx, testGenesis.StateRoot, &i))
}

func TestRejectBadChain(t *testing.T) {
	assert := assert.New(t)
	ctx, cs, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	addBlocks(t, cs, bad1)
	res, err := stm.ProcessNewBlock(ctx, bad2)
	assert.EqualError(err, ErrInvalidBase.Error())
	assert.Equal(InvalidBase, res)
	assert.Equal(stm.GetBestBlock(), testGenesis)
}

func TestRejectDiffHeightParents(t *testing.T) {
	assert := assert.New(t)
	ctx, cs, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	addBlocks(t, cs, bad3, tipsetA1, block1, block2)
	res, err := stm.ProcessNewBlock(ctx, bad3)
	assert.Error(err)
	assert.Equal(Unknown, res)
	assert.Equal(stm.GetBestBlock(), testGenesis)
}

func TestBlockHistory(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	requireProcessBlock(ctx, t, stm, block1)
	requireProcessBlock(ctx, t, stm, block2)

	tsCh := stm.BlockHistory(ctx)

	assert.Equal(NewTipSet(block2), ((<-tsCh).(TipSet)))
	assert.Equal(NewTipSet(block1), ((<-tsCh).(TipSet)))
	assert.Equal(stm.GetGenesisCid(), ((<-tsCh).(TipSet)).ToSlice()[0].Cid())
	ts, more := <-tsCh
	assert.Equal(nil, ts)     // Genesis block has no parent.
	assert.Equal(false, more) // Channel is closed
}

func TestBlockHistoryFetchError(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	requireProcessBlock(ctx, t, stm, block1)
	requireProcessBlock(ctx, t, stm, block2)

	tsCh := stm.BlockHistory(ctx)

	stm.FetchBlock = func(ctx context.Context, cid *cid.Cid) (*types.Block, error) {
		return nil, fmt.Errorf("error fetching block (in test)")
	}
	// One tipset is already ready.
	assert.Equal(NewTipSet(block2), ((<-tsCh).(TipSet)))

	// Next tipset sent should instead be an error.
	next := <-tsCh

	_, ok := next.(error)
	assert.True(ok)
}

func TestBlockHistoryCancel(t *testing.T) {
	assert := assert.New(t)
	_, _, _, stm := newTestUtils()
	ctx, cancel := context.WithCancel(context.Background())

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	requireProcessBlock(ctx, t, stm, block1)
	requireProcessBlock(ctx, t, stm, block2)

	tsCh := stm.BlockHistory(ctx)
	assert.Equal(NewTipSet(block2), ((<-tsCh).(TipSet)))
	cancel()
	time.Sleep(10 * time.Millisecond)

	ts, more := <-tsCh
	// Channel is closed
	assert.Equal(nil, ts)
	assert.Equal(false, more)
}

func assertPut(assert *assert.Assertions, cst *hamt.CborIpldStore, i interface{}) {
	_, err := cst.Put(context.TODO(), i)
	assert.NoError(err)
}

func TestChainLoad(t *testing.T) {
	assert := assert.New(t)
	_, cs, ds, stm := newTestUtils()

	assert.NoError(putCidSet(context.Background(), ds, heaviestTipSetKey, types.NewSortedCidSet(block2.Cid())))

	assertPut(assert, cs, testGenesis)
	assertPut(assert, cs, block1)
	assertPut(assert, cs, block2)

	assert.NoError(stm.Load())

	assert.Equal(block2, stm.GetBestBlock())
	assert.Equal(testGenesis.Cid(), stm.GetGenesisCid())
}

func TestChainLoadMultiBlockTipSet(t *testing.T) {
	assert := assert.New(t)
	_, cs, ds, stm := newTestUtils()

	assert.NoError(putCidSet(context.Background(), ds, heaviestTipSetKey, tipsetB.ToSortedCidSet()))

	assertPut(assert, cs, testGenesis)
	addBlocks(t, cs, tipsetA.ToSlice()...)
	addBlocks(t, cs, tipsetB.ToSlice()...)
	assert.NoError(stm.Load())

	assert.Equal(tipsetB, stm.GetHeaviestTipSet())
	assert.Equal(testGenesis.Cid(), stm.GetGenesisCid())
}

func TestTipSets(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()

	assert.Len(stm.GetTipSetsByHeight(0), 0)
	assert.NoError(stm.Genesis(ctx, InitGenesis))
	assert.Len(stm.GetTipSetsByHeight(0), 1)
	requireProcessBlock(ctx, t, stm, block1)
	assert.Len(stm.GetTipSetsByHeight(1), 1)

	// add in more forks
	requireProcessBlock(ctx, t, stm, block2)

	requireProcessBlock(ctx, t, stm, tipsetA1)
	requireProcessBlock(ctx, t, stm, tipsetA2)
	requireProcessBlock(ctx, t, stm, tipsetA3)

	requireProcessBlock(ctx, t, stm, tipsetB1)
	requireProcessBlock(ctx, t, stm, tipsetB2)

	requireProcessBlock(ctx, t, stm, tipsetFork)

	assert.Len(stm.GetTipSetsByHeight(1), 1)
	assert.Len(stm.GetTipSetsByHeight(2), 3) // tipsetB, {block2}, {tipsetFork}
}
