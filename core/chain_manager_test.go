package core

import (
	"context"
	"testing"

	hamt "gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
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

	block1 = mkChild(testGenesis, 0)
	block2 = mkChild(block1, 0)

	fork1 = mkChild(testGenesis, 1)
	fork2 = mkChild(fork1, 1)
	fork3 = mkChild(fork2, 1)

	bad1 = &types.Block{
		StateRoot: testGenesis.StateRoot,
		Nonce:     404,
	}
	bad2 = mkChild(bad1, 0)
}

func mkChild(blk *types.Block, nonce uint64) *types.Block {
	return &types.Block{
		Parent:          blk.Cid(),
		Height:          blk.Height + 1,
		Nonce:           nonce,
		StateRoot:       blk.StateRoot,
		Messages:        []*types.Message{},
		MessageReceipts: []*types.MessageReceipt{},
	}
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
	stm := NewChainManager(cs)

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

func TestBestBlockCh(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()

	// Make sure it doesn't blow up with a nil channel.
	stm := NewChainManager(cs)
	assert.NoError(stm.Genesis(ctx, InitGenesis))
	stm.SetBestBlockCh(nil)
	assert.NotPanics(func() { stm.ProcessNewBlock(ctx, block1) })

	// Check that it works.
	ch := make(chan *types.Block)
	stm = NewChainManager(cs)
	assert.NoError(stm.Genesis(ctx, InitGenesis))
	stm.SetBestBlockCh(ch)
	block3 := mkChild(block2, 0)
	blocks := []*types.Block{block1, block2, block3}
	for _, b := range blocks {
		stm.ProcessNewBlock(ctx, b)
	}
	gotCids := map[string]bool{}
	for range blocks {
		gotBlock := <-ch
		gotCids[gotBlock.Cid().String()] = true
	}
	for _, b := range blocks {
		assert.True(gotCids[b.Cid().String()])
	}
}

func TestBestBlockPubSub(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()

	stm := NewChainManager(cs)
	ch := stm.BestBlockPubSub.Sub(BlockTopic)

	assert.NoError(stm.Genesis(ctx, InitGenesis))
	block3 := mkChild(block2, 0)
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
	stm := NewChainManager(cs)

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
}

func TestRejectShorterChain(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	stm := NewChainManager(cs)

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
	stm := NewChainManager(cs)

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
	stm := NewChainManager(cs)

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
	stm := NewChainManager(cs)

	assert.NoError(stm.Genesis(ctx, InitGenesis))

	addBlocks(t, cs, bad1)
	res, err := stm.ProcessNewBlock(ctx, bad2)
	assert.EqualError(err, ErrInvalidBase.Error())
	assert.Equal(InvalidBase, res)
	assert.Equal(stm.GetBestBlock(), testGenesis)
}
