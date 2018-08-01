package core

import (
	"context"
	hamt "gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"
	datastore "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	"testing"

	"github.com/filecoin-project/go-filecoin/state"

	"github.com/stretchr/testify/assert"
)

func countBlocks(chm *ChainManager) (count int) {
	for range chm.BlockHistory(context.Background()) {
		count++
	}
	return count
}

func TestAddChain(t *testing.T) {
	assert := assert.New(t)
	cs := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	chm := NewChainManager(ds, cs)
	chm.PwrTableView = &TestView{}

	ctx := context.Background()
	assert.NoError(chm.Genesis(ctx, InitGenesis))

	assert.Equal(1, countBlocks(chm))

	bts := chm.GetHeaviestTipSet()
	stateGetter := func(ctx context.Context, ts TipSet) (state.Tree, error) {
		return chm.State(ctx, ts.ToSlice())
	}
	AddChain(ctx, chm.ProcessNewBlock, stateGetter, bts.ToSlice(), 9)

	assert.Equal(10, countBlocks(chm))
}
