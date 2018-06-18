package core

import (
	"context"
	datastore "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	hamt "gx/ipfs/QmcYBp5EDnJKfVN63F71rDTksvEf1cfijwCTWtw6bPG58T/go-hamt-ipld"
	"testing"

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

	ctx := context.Background()
	assert.NoError(chm.Genesis(ctx, InitGenesis))

	assert.Equal(1, countBlocks(chm))

	bb := chm.GetBestBlock()
	AddChain(ctx, chm.ProcessNewBlock, bb, 9)

	assert.Equal(10, countBlocks(chm))
}
