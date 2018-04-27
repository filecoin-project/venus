package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

var (
	testGenesis, block1, block2, fork1, fork2, fork3 *types.Block

	bad1, bad2 *types.Block
)

func init() {
	cst := hamt.NewCborStore()
	genesis, err := InitGenesis(cst)
	if err != nil {
		panic(err)
	}
	testGenesis = genesis

	block1 = MkChild(testGenesis, 0)
	block2 = MkChild(block1, 0)

	fork1 = MkChild(testGenesis, 1)
	fork2 = MkChild(fork1, 1)
	fork3 = MkChild(fork2, 1)

	bad1 = &types.Block{
		StateRoot: testGenesis.StateRoot,
		Nonce:     404,
	}
	bad2 = MkChild(bad1, 0)
}

func addBlocks(t *testing.T, cs *hamt.CborIpldStore, blks ...*types.Block) {
	for _, blk := range blks {
		_, err := cs.Put(context.Background(), blk)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestBasicAddBlock(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	res, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block1.Cid())
	assert.True(stm.knownGoodBlocks.Has(block1.Cid()))

	res, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block2.Cid())
	assert.True(stm.knownGoodBlocks.Has(block2.Cid()))
}

func TestBestBlockPubSub(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()

	stm := NewChainManager(ds, cs)
	ch := stm.BestBlockPubSub.Sub(BlockTopic)

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	block3 := MkChild(block2, 0)
	blocks := []*types.Block{block1, block2, block3}
	for _, b := range blocks {
		stm.ProcessNewBlock(ctx, b)
	}
	gotCids := map[string]bool{}
	for i := 0; i < 4; i++ {
		gotBlock := <-ch
		gotCids[gotBlock.(*types.Block).Cid().String()] = true
	}
	for _, b := range blocks {
		assert.True(gotCids[b.Cid().String()])
	}
}

func TestForkChoice(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	res, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block1.Cid())
	assert.True(stm.knownGoodBlocks.Has(block1.Cid()))

	// progress to block2 block on our chain
	res, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block2.Cid())
	assert.True(stm.knownGoodBlocks.Has(block2.Cid()))

	// Now, introduce a valid fork
	addBlocks(t, cs, fork1, fork2)

	res, err = stm.ProcessNewBlock(ctx, fork3)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), fork3.Cid())

	bbc, err := stm.readBestBlockCid()
	assert.NoError(err)
	assert.Equal(fork3.Cid(), bbc)
}

func TestRejectShorterChain(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	res, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block1.Cid())

	res, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block2.Cid())

	// block with lower height than our current shouldnt fail, but it shouldnt be accepted as the best block
	res, err = stm.ProcessNewBlock(ctx, fork1)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block2.Cid())

	// block with same height as our current should fail
	res, err = stm.ProcessNewBlock(ctx, fork2)
	assert.NoError(err)
	assert.Equal(ChainValid, res)
	assert.Equal(stm.bestBlock.blk.Cid(), block2.Cid())
}

func TestKnownAncestor(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	addBlocks(t, cs, block1)
	res, err := stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)
	assert.Equal(ChainAccepted, res)

	addBlocks(t, cs, fork1, fork2)
	base, chain, err := stm.findKnownAncestor(ctx, fork3)
	assert.NoError(err)
	assert.Equal(testGenesis, base)
	assert.Len(chain, 3)
	assert.Equal(fork3, chain[0])
	assert.Equal(fork2, chain[1])
	assert.Equal(fork1, chain[2])
}

func TestGenesis(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	assert.Equal(testGenesis, stm.bestBlock.blk)
	assert.True(stm.knownGoodBlocks.Has(testGenesis.Cid()))

	var i interface{}
	assert.NoError(stm.cstore.Get(ctx, testGenesis.StateRoot, &i))
}

func TestRejectBadChain(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	addBlocks(t, cs, bad1)
	res, err := stm.ProcessNewBlock(ctx, bad2)
	assert.EqualError(err, ErrInvalidBase.Error())
	assert.Equal(InvalidBase, res)
	assert.Equal(stm.GetBestBlock(), testGenesis)
}

func TestBlockHistory(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	_, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)

	_, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)

	blockCh := stm.BlockHistory(ctx)

	assert.True(block2.Equals((<-blockCh).(*types.Block)))
	assert.True(block1.Equals((<-blockCh).(*types.Block)))
	assert.Equal(stm.GetGenesisCid(), ((<-blockCh).(*types.Block)).Cid())
	blk, more := <-blockCh
	assert.Equal(nil, blk)    // Genesis block has no parent.
	assert.Equal(false, more) // Channel is closed
}

func TestBlockHistoryFetchError(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	_, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)

	_, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)

	blockCh := stm.BlockHistory(ctx)

	stm.FetchBlock = func(ctx context.Context, cid *cid.Cid) (*types.Block, error) {
		return nil, fmt.Errorf("error fetching block (in test)")
	}
	// One block is already ready.
	assert.True(block2.Equals((<-blockCh).(*types.Block)))

	// Next block sent should instead be an error.
	next := <-blockCh

	_, ok := next.(error)
	assert.True(ok)
}

func TestBlockHistoryCancel(t *testing.T) {
	assert := assert.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	_, err := stm.ProcessNewBlock(ctx, block1)
	assert.NoError(err)

	_, err = stm.ProcessNewBlock(ctx, block2)
	assert.NoError(err)

	blockCh := stm.BlockHistory(ctx)
	assert.True(block2.Equals((<-blockCh).(*types.Block)))
	cancel()
	time.Sleep(10 * time.Millisecond)

	blk, more := <-blockCh
	// Channel is closed
	assert.Equal(nil, blk)
	assert.Equal(false, more)
}

func assertPut(assert *assert.Assertions, cst *hamt.CborIpldStore, i interface{}) {
	_, err := cst.Put(context.TODO(), i)
	assert.NoError(err)

}
func TestChainLoad(t *testing.T) {
	assert := assert.New(t)
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	stm := NewChainManager(ds, cs)

	assert.NoError(putCid(context.Background(), ds, bestBlockKey, block2.Cid()))

	assertPut(assert, cs, testGenesis)
	assertPut(assert, cs, block1)
	assertPut(assert, cs, block2)

	assert.NoError(stm.Load())

	assert.Equal(block2, stm.GetBestBlock())
	assert.Equal(testGenesis.Cid(), stm.GetGenesisCid())
}

func TestChainManagerInformNewBlock(t *testing.T) {
	makeGetBestBlockFunc := func(calls *[]string, blk *types.Block) func() *types.Block {
		return func() *types.Block {
			*calls = append(*calls, "GetBestBlock")
			return blk
		}
	}

	makeProcessNewBlockFunc := func(calls *[]string, bpr BlockProcessResult, err error) func(context.Context, *types.Block) (BlockProcessResult, error) {
		return func(_ context.Context, _ *types.Block) (BlockProcessResult, error) {
			*calls = append(*calls, "ProcessNewBlock")
			return bpr, err
		}
	}

	makeFetchBlockFunc := func(calls *[]string, blk *types.Block, err error) func(_ context.Context, _ *cid.Cid) (*types.Block, error) {
		return func(_ context.Context, _ *cid.Cid) (*types.Block, error) {
			*calls = append(*calls, "FetchBlock")
			return blk, err
		}
	}

	t.Run("informNewBlock does not process new block if its height is less than current best block's height", func(t *testing.T) {
		assert := assert.New(t)

		var newBlockHeight uint64 = 1

		var calls []string
		deps := informNewBlockDeps{
			GetBestBlock:    makeGetBestBlockFunc(&calls, &types.Block{Height: newBlockHeight + 1}),
			FetchBlock:      makeFetchBlockFunc(&calls, nil, nil),
			ProcessNewBlock: makeProcessNewBlockFunc(&calls, 0, nil),
		}

		(&ChainManager{}).informNewBlock(deps, "foo", nil, newBlockHeight)

		assert.Equal([]string{"GetBestBlock"}, calls)
	})

	t.Run("informNewBlock does not process new block if error fetching block by cid", func(t *testing.T) {
		assert := assert.New(t)

		var calls []string

		deps := informNewBlockDeps{
			GetBestBlock:    makeGetBestBlockFunc(&calls, &types.Block{Height: 1}),
			FetchBlock:      makeFetchBlockFunc(&calls, nil, errors.New("error")),
			ProcessNewBlock: makeProcessNewBlockFunc(&calls, 0, nil),
		}

		(&ChainManager{}).informNewBlock(deps, "foo", nil, 2)

		assert.Equal([]string{"GetBestBlock", "FetchBlock"}, calls)
	})

	t.Run("informNewBlock does not panic when ProcessNewBlock returns an error", func(t *testing.T) {
		assert := assert.New(t)

		var calls []string

		deps := informNewBlockDeps{
			GetBestBlock:    makeGetBestBlockFunc(&calls, &types.Block{Height: 1}),
			FetchBlock:      makeFetchBlockFunc(&calls, &types.Block{Height: 2}, nil),
			ProcessNewBlock: makeProcessNewBlockFunc(&calls, 0, errors.New("error")),
		}

		(&ChainManager{}).informNewBlock(deps, "foo", nil, 2)

		assert.Equal([]string{"GetBestBlock", "FetchBlock", "ProcessNewBlock"}, calls)
	})
}
