package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	hamt "gx/ipfs/QmSwABWvsucRwH7XVDbTE3aJgiVdtJUfeWjY8oejB4RmAA/go-hamt-ipld"

	types "github.com/filecoin-project/go-filecoin/types"
)

var (
	testGenesis = &types.Block{}

	block1 = &types.Block{
		Parent: testGenesis.Cid(),
		Height: 1,
	}

	block2 = &types.Block{
		Parent: block1.Cid(),
		Height: 2,
	}

	fork1 = &types.Block{
		Parent: testGenesis.Cid(),
		Height: 1,
		Nonce:  1,
	}

	fork2 = &types.Block{
		Parent: fork1.Cid(),
		Height: 2,
	}

	fork3 = &types.Block{
		Parent: fork2.Cid(),
		Height: 3,
	}
)

func TestBasicAddBlock(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	stm := NewStateManager(cs)

	assert.NoError(stm.SetBestBlock(ctx, testGenesis))

	assert.NoError(stm.ProcessNewBlock(ctx, block1))
	assert.Equal(stm.BestBlock.Cid(), block1.Cid())

	assert.NoError(stm.ProcessNewBlock(ctx, block2))
	assert.Equal(stm.BestBlock.Cid(), block2.Cid())
}

func TestForkChoice(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	stm := NewStateManager(cs)

	assert.NoError(stm.SetBestBlock(ctx, testGenesis))

	assert.NoError(stm.ProcessNewBlock(ctx, block1))
	assert.Equal(stm.BestBlock.Cid(), block1.Cid())

	// progress to block1 block on our chain
	assert.NoError(stm.ProcessNewBlock(ctx, block2))
	assert.Equal(stm.BestBlock.Cid(), block2.Cid())

	// Now, introduce a valid fork
	_, err := cs.Put(ctx, fork1)
	// TODO: when checking blocks, we should probably hold onto them for a
	// period of time. For now we can be okay dropping them, but later this
	// will be important.
	assert.NoError(err)

	_, err = cs.Put(ctx, fork2)
	assert.NoError(err)

	assert.NoError(stm.ProcessNewBlock(ctx, fork3))
	assert.Equal(stm.BestBlock.Cid(), fork3.Cid())
}

func TestRejectShorterChain(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	cs := hamt.NewCborStore()
	stm := NewStateManager(cs)

	assert.NoError(stm.SetBestBlock(ctx, testGenesis))

	assert.NoError(stm.ProcessNewBlock(ctx, block1))
	assert.Equal(stm.BestBlock.Cid(), block1.Cid())

	assert.NoError(stm.ProcessNewBlock(ctx, block2))
	assert.Equal(stm.BestBlock.Cid(), block2.Cid())

	// block with same height as our current should fail
	assert.EqualError(stm.ProcessNewBlock(ctx, fork1), "validate block failed: new block is not better than our current block")
	assert.Equal(stm.BestBlock.Cid(), block2.Cid())

	// block with same height as our current should fail
	assert.EqualError(stm.ProcessNewBlock(ctx, fork2), "validate block failed: new block is not better than our current block")
	assert.Equal(stm.BestBlock.Cid(), block2.Cid())
}
