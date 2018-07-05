package core

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testGenesis, block1, block2, fork1, fork2, fork3             *types.Block
	tipsetA1, tipsetA2, tipsetA3, tipsetB1, tipsetB2, tipsetFork *types.Block

	bad1, bad2, bad3 *types.Block
)

func init() {
	cst := hamt.NewCborStore()
	r := repo.NewInMemoryRepo()
	ds := r.Datastore()

	genesis, err := InitGenesis(cst, ds)
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

	tipsetB1 = MkChild([]*types.Block{tipsetA1, tipsetA2, tipsetA3}, genesis.StateRoot, 0)
	tipsetB2 = MkChild([]*types.Block{tipsetA1, tipsetA2, tipsetA3}, genesis.StateRoot, 1)

	tipsetFork = MkChild([]*types.Block{tipsetA1, tipsetA3}, genesis.StateRoot, 0)

	bad1 = &types.Block{
		StateRoot: testGenesis.StateRoot,
		Nonce:     404,
	}
	bad2 = MkChild([]*types.Block{bad1}, genesis.StateRoot, 0)

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
	cm := NewChainManager(ds, cs)
	cm.PwrTableView = &TestView{}
	return ctx, cs, ds, cm
}

func requireProcessBlock(ctx context.Context, t *testing.T, cm *ChainManager, b *types.Block) {
	require := require.New(t)
	_, err := cm.ProcessNewBlock(ctx, b)
	require.NoError(err)
}

func TestBasicAddBlock(t *testing.T) {
	ctx, _, _, cm := newTestUtils()
	assert := assert.New(t)

	assert.NoError(cm.Genesis(ctx, InitGenesis))
	res, err := cm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	assert.Equal(RequireBestBlock(cm, t).Cid(), block1.Cid())
	assert.True(cm.knownGoodBlocks.Has(block1.Cid()))
	res, err = cm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block2.Cid())
	assert.True(cm.knownGoodBlocks.Has(block2.Cid()))
}

func TestMultiBlockTipsetAdd(t *testing.T) {
	ctx, _, _, cm := newTestUtils()
	assert := assert.New(t)
	require := require.New(t)

	// Add tipsetA
	tipsetA := RequireNewTipSet(require, tipsetA1, tipsetA2, tipsetA3)
	assert.NoError(cm.Genesis(ctx, InitGenesis))
	res, err := cm.ProcessNewBlock(ctx, tipsetA1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	res, err = cm.ProcessNewBlock(ctx, tipsetA2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	res, err = cm.ProcessNewBlock(ctx, tipsetA3)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	assert.Equal(cm.GetHeaviestTipSet(), tipsetA)

	// Add tipsetB
	tipsetB := RequireNewTipSet(require, tipsetB1, tipsetB2)
	res, err = cm.ProcessNewBlock(ctx, tipsetB1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), tipsetB1.Cid())
	res, err = cm.ProcessNewBlock(ctx, tipsetB2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	assert.Equal(cm.GetHeaviestTipSet(), tipsetB)
}

func TestHeaviestTipSetPubSub(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, _, _, cm := newTestUtils()
	ch := cm.HeaviestTipSetPubSub.Sub(HeaviestTipSetTopic)

	assert.NoError(cm.Genesis(ctx, InitGenesis))
	block3 := MkChild([]*types.Block{block2}, block2.StateRoot, 0)
	block4 := MkChild([]*types.Block{block2}, block2.StateRoot, 1)
	blocks := []*types.Block{block1, block2, block3, block4}
	expTipSets := []TipSet{
		RequireNewTipSet(require, block1),
		RequireNewTipSet(require, block2),
		RequireNewTipSet(require, block3), // intermediate tipset is published
		RequireNewTipSet(require, block3, block4),
	}
	for _, b := range blocks {
		cm.ProcessNewBlock(ctx, b)
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
	ctx, cs, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	res, err := cm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block1.Cid())
	assert.True(cm.knownGoodBlocks.Has(block1.Cid()))

	// progress to block2 block on our chain
	res, err = cm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block2.Cid())
	assert.True(cm.knownGoodBlocks.Has(block2.Cid()))

	// Now, introduce a valid fork
	addBlocks(t, cs, fork1, fork2)

	res, err = cm.ProcessNewBlock(ctx, fork3)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), fork3.Cid())

	cids, err := cm.readHeaviestTipSetCids()
	assert.NoError(err)
	assert.Equal(cids.Len(), 1)
	bbc := cids.Iter().Value()
	assert.Equal(fork3.Cid(), bbc)
}

func TestMultiBlockTipsetForkChoice(t *testing.T) {
	ctx, _, _, cm := newTestUtils()
	assert := assert.New(t)
	require := require.New(t)

	// Progress to tipsetB
	tipsetB := RequireNewTipSet(require, tipsetB1, tipsetB2)
	assert.NoError(cm.Genesis(ctx, InitGenesis))
	requireProcessBlock(ctx, t, cm, tipsetA1)
	requireProcessBlock(ctx, t, cm, tipsetA2)
	requireProcessBlock(ctx, t, cm, tipsetA3)
	requireProcessBlock(ctx, t, cm, tipsetB1)
	assert.Equal(RequireBestBlock(cm, t).Cid(), tipsetB1.Cid())
	requireProcessBlock(ctx, t, cm, tipsetB2)
	assert.Equal(cm.GetHeaviestTipSet(), tipsetB)

	// Introduce a losing fork
	res, err := cm.ProcessNewBlock(ctx, tipsetFork)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(cm.GetHeaviestTipSet(), tipsetB)
	cids, err := cm.readHeaviestTipSetCids()
	assert.NoError(err)
	assert.True(cids.Equals(tipsetB.ToSortedCidSet()))
}

func TestRejectShorterChain(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	res, err := cm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block1.Cid())

	res, err = cm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block2.Cid())

	// block with lower height than our current shouldnt fail, but it shouldnt be accepted as the best block
	res, err = cm.ProcessNewBlock(ctx, fork1)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block2.Cid())

	// block with same height as our current should fail
	res, err = cm.ProcessNewBlock(ctx, fork2)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(RequireBestBlock(cm, t).Cid(), block2.Cid())
}

func TestGenesis(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))
	assert.Equal(testGenesis, cm.heaviestTipSet.ts.ToSlice()[0])
	assert.True(cm.knownGoodBlocks.Has(testGenesis.Cid()))

	var i interface{}
	assert.NoError(cm.cstore.Get(ctx, testGenesis.StateRoot, &i))
}

func TestRejectBadChain(t *testing.T) {
	assert := assert.New(t)
	ctx, cs, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	addBlocks(t, cs, bad1)
	res, err := cm.ProcessNewBlock(ctx, bad2)
	assert.EqualError(err, ErrInvalidBase.Error())
	assert.Equal(InvalidBase, res)
	assert.Equal(RequireBestBlock(cm, t), testGenesis)
}

func TestRejectDiffHeightParents(t *testing.T) {
	assert := assert.New(t)
	ctx, cs, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	addBlocks(t, cs, bad3, tipsetA1, block1, block2)
	res, err := cm.ProcessNewBlock(ctx, bad3)
	assert.Error(err)
	assert.Equal(Unknown, res)
	assert.Equal(RequireBestBlock(cm, t), testGenesis)
}

func TestBlockHistory(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, _, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	requireProcessBlock(ctx, t, cm, block1)
	requireProcessBlock(ctx, t, cm, block2)

	tsCh := cm.BlockHistory(ctx)

	assert.Equal(RequireNewTipSet(require, block2), ((<-tsCh).(TipSet)))
	assert.Equal(RequireNewTipSet(require, block1), ((<-tsCh).(TipSet)))
	assert.Equal(cm.GetGenesisCid(), ((<-tsCh).(TipSet)).ToSlice()[0].Cid())
	ts, more := <-tsCh
	assert.Equal(nil, ts)     // Genesis block has no parent.
	assert.Equal(false, more) // Channel is closed
}

func TestBlockHistoryFetchError(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx, _, _, cm := newTestUtils()

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	requireProcessBlock(ctx, t, cm, block1)
	requireProcessBlock(ctx, t, cm, block2)

	tsCh := cm.BlockHistory(ctx)

	cm.FetchBlock = func(ctx context.Context, cid *cid.Cid) (*types.Block, error) {
		return nil, fmt.Errorf("error fetching block (in test)")
	}
	// One tipset is already ready.
	assert.Equal(RequireNewTipSet(require, block2), ((<-tsCh).(TipSet)))

	// Next tipset sent should instead be an error.
	next := <-tsCh

	_, ok := next.(error)
	assert.True(ok)
}

func TestBlockHistoryCancel(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	_, _, _, cm := newTestUtils()
	ctx, cancel := context.WithCancel(context.Background())

	assert.NoError(cm.Genesis(ctx, InitGenesis))

	requireProcessBlock(ctx, t, cm, block1)
	requireProcessBlock(ctx, t, cm, block2)

	tsCh := cm.BlockHistory(ctx)
	assert.Equal(RequireNewTipSet(require, block2), ((<-tsCh).(TipSet)))
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
	_, cs, ds, cm := newTestUtils()

	assert.NoError(putCidSet(context.Background(), ds, heaviestTipSetKey, types.NewSortedCidSet(block2.Cid())))

	assertPut(assert, cs, testGenesis)
	assertPut(assert, cs, block1)
	assertPut(assert, cs, block2)

	assert.NoError(cm.Load())

	assert.Equal(block2, RequireBestBlock(cm, t))
	assert.Equal(testGenesis.Cid(), cm.GetGenesisCid())
}

func TestChainLoadMultiBlockTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	_, cs, ds, cm := newTestUtils()

	tipsetB := RequireNewTipSet(require, tipsetB1, tipsetB2)
	assert.NoError(putCidSet(context.Background(), ds, heaviestTipSetKey, tipsetB.ToSortedCidSet()))

	assertPut(assert, cs, testGenesis)
	addBlocks(t, cs, tipsetA1, tipsetA2, tipsetA3)
	addBlocks(t, cs, tipsetB1, tipsetB2)
	assert.NoError(cm.Load())

	assert.Equal(tipsetB, cm.GetHeaviestTipSet())
	assert.Equal(testGenesis.Cid(), cm.GetGenesisCid())
}

func TestTipSets(t *testing.T) {
	assert := assert.New(t)
	ctx, _, _, cm := newTestUtils()

	assert.Len(cm.GetTipSetsByHeight(0), 0)
	assert.NoError(cm.Genesis(ctx, InitGenesis))
	assert.Len(cm.GetTipSetsByHeight(0), 1)
	requireProcessBlock(ctx, t, cm, block1)
	assert.Len(cm.GetTipSetsByHeight(1), 1)

	// add in more forks
	requireProcessBlock(ctx, t, cm, block2)

	requireProcessBlock(ctx, t, cm, tipsetA1)
	requireProcessBlock(ctx, t, cm, tipsetA2)
	requireProcessBlock(ctx, t, cm, tipsetA3)

	requireProcessBlock(ctx, t, cm, tipsetB1)
	requireProcessBlock(ctx, t, cm, tipsetB2)

	requireProcessBlock(ctx, t, cm, tipsetFork)

	assert.Len(cm.GetTipSetsByHeight(1), 1)
	assert.Len(cm.GetTipSetsByHeight(2), 3) // tipsetB, {block2}, {tipsetFork}
}

// TestTipSetWeightShallow does not set up storage commitments in the state of
// the chain but checks that block processing works when the chain managers
// sees all miners as having 0 power and the total storage being 1 byte.
func TestTipSetWeightShallow(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	ctx, _, _, cm := newTestUtils()
	assert.NoError(cm.Genesis(ctx, InitGenesis))
	requireProcessBlock(ctx, t, cm, tipsetA1)
	requireProcessBlock(ctx, t, cm, tipsetA2)
	requireProcessBlock(ctx, t, cm, tipsetA3)
	requireProcessBlock(ctx, t, cm, tipsetB1)
	requireProcessBlock(ctx, t, cm, tipsetB2)

	ts := RequireNewTipSet(require, tipsetB1, tipsetB2)
	w, _, err := cm.Weight(ctx, ts)
	assert.NoError(err)
	assert.Equal(uint64(ECV*6), w)
}

// TestTipSetWeightDeepFailure tests the behavior of the Chain Manager with
// the production power table view over failure blocks
func TestTipSetWeightDeepFailure(t *testing.T) {
	//	require := require.New(t)
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()
	stm.PwrTableView = &marketView{}

	// process a block without setting up the genesis block
	res, err := stm.ProcessNewBlock(ctx, testGenesis)
	assert.Equal(Uninit, res)
	assert.EqualError(err, ErrUninit.Error())

	// genesis block does not have a power table set up
	assert.NoError(stm.Genesis(ctx, InitGenesis))
	_, err = stm.ProcessNewBlock(ctx, block1)
	assert.Error(err)

	// child block with an invalid state cid
	newCid := types.NewCidForTestGetter()
	badChild := MkChild([]*types.Block{testGenesis}, newCid(), 0)
	_, err = stm.ProcessNewBlock(ctx, badChild)
	assert.Error(err)
}

// TestTipSetWeightDeep sets up a genesis block with storage commitments and
// tests that power fractions are accurately calculated.
func TestTipSetWeightDeep(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)
	ctx, _, _, stm := newTestUtils()
	stm.PwrTableView = &TestView{}

	// setup miner power in genesis block
	ki := types.MustGenerateKeyInfo(1, types.GenerateKeyInfoSeed())
	mockSigner := types.NewMockSigner(ki)
	testAddress := mockSigner.Addresses[0]

	// pwr1, pwr2 = 1/100. pwr3 = 98/100.
	pwr1, pwr2, pwr3 := types.NewBytesAmount(10000), types.NewBytesAmount(10000), types.NewBytesAmount(980000)
	testGen := th.MakeGenesisFunc(
		th.ActorAccount(testAddress, types.NewAttoFILFromFIL(10000)),
	)
	require.NoError(stm.Genesis(ctx, testGen))

	genesisBlock, err := stm.FetchBlock(ctx, stm.genesisCid)
	require.NoError(err)
	addr0, block, nonce, err := CreateMinerWithPower(ctx, t, stm, genesisBlock, mockSigner, 0, mockSigner.Addresses[0], nil)
	require.NoError(err)
	addr1, block, nonce, err := CreateMinerWithPower(ctx, t, stm, block, mockSigner, nonce, addr0, pwr1)
	require.NoError(err)
	addr2, block, nonce, err := CreateMinerWithPower(ctx, t, stm, block, mockSigner, nonce, addr0, pwr2)
	require.NoError(err)
	addr3, _, _, err := CreateMinerWithPower(ctx, t, stm, block, mockSigner, nonce, addr0, pwr3)
	require.NoError(err)

	stm.PwrTableView = &marketView{}

	heaviest := stm.GetHeaviestTipSet()
	require.Equal(1, len(heaviest))
	gen := heaviest.ToSlice()[0]

	/* Test chain diagram and weight calcs */
	// (Note f1b1 = fork 1 block 1)
	//
	// f1b1 -> {f1b2a, f1b2b}
	//
	// f2b1 -> f2b2
	//
	//  w({blk})          = sw + apw + mw      = sw + ew
	//  w({fXb1})         = sw + 0   + 11      = sw + 11
	//  w({f1b1, f2b1})   = sw + 0   + 11 * 2  = sw + 22
	//  w({f1b2a, f1b2b}) = sw + 11  + 11 * 2  = sw + 33
	//  w({f2b2})         = sw + 11  + 108 	   = sw + 119
	//  sw=starting weight, apw=added parent weight, mw=miner weight, ew=expected weight
	//  mw = 10*(mp/tp)+10 = 10*(10000,980000)/1000000+10
	startingWeight, err := stm.weight(ctx, heaviest)
	assert.NoError(err)

	f1b1 := requireMkBlockCorrected(ctx, t, stm, 0, gen)
	f1b1.Miner = addr1
	f2b1 := requireMkBlockCorrected(ctx, t, stm, 1, gen)
	f2b1.Miner = addr2
	tsBase := RequireNewTipSet(require, f1b1, f2b1)

	requireProcessBlock(ctx, t, stm, f1b1)
	requireProcessBlock(ctx, t, stm, f2b1)
	heaviest = stm.GetHeaviestTipSet()
	assert.Equal(tsBase, heaviest)
	w, err := stm.weight(ctx, heaviest)
	assert.NoError(err)

	expectedWeight := big.NewRat(int64(22), int64(1))
	expectedWeight.Add(expectedWeight, startingWeight)
	require.Equal(expectedWeight, w)

	// fork 1 is heavier than the old head.
	f1b2a := requireMkBlockCorrected(ctx, t, stm, 0, f1b1)
	f1b2a.Miner = addr1

	f1b2b := requireMkBlockCorrected(ctx, t, stm, 1, f1b1)
	f1b2b.Miner = addr2
	f1 := RequireNewTipSet(require, f1b2a, f1b2b)

	requireProcessBlock(ctx, t, stm, f1b2a)
	requireProcessBlock(ctx, t, stm, f1b2b)
	heaviest = stm.GetHeaviestTipSet()
	assert.Equal(f1, heaviest)
	w, err = stm.weight(ctx, heaviest)
	assert.NoError(err)
	expectedWeight = big.NewRat(int64(33), int64(1))
	expectedWeight.Add(expectedWeight, startingWeight)
	require.Equal(expectedWeight, w)

	// fork 2 has heavier weight because of addr3's power even though there
	// are fewer blocks in the tipset than fork 1
	f2b2 := requireMkBlockCorrected(ctx, t, stm, 0, f2b1)
	f2b2.Miner = addr3
	f2 := RequireNewTipSet(require, f2b2)

	requireProcessBlock(ctx, t, stm, f2b2)
	heaviest = stm.GetHeaviestTipSet()
	assert.Equal(f2, heaviest)
	w, err = stm.weight(ctx, heaviest)
	assert.NoError(err)
	expectedWeight = big.NewRat(int64(119), int64(1))
	expectedWeight.Add(expectedWeight, startingWeight)
	assert.Equal(expectedWeight, w)
}

func requireMkBlockCorrected(ctx context.Context, t *testing.T, cm *ChainManager, nonce uint64, parents ...*types.Block) *types.Block {
	tipSet, err := NewTipSet(parents...)
	require.NoError(t, err)

	weight, err := cm.weight(ctx, tipSet)
	require.NoError(t, err)

	blk := MkChild(parents, parents[0].StateRoot, nonce)
	blk.ParentWeightNum = types.Uint64(weight.Num().Uint64())
	blk.ParentWeightDenom = types.Uint64(weight.Denom().Uint64())
	return blk
}
