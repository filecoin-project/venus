package main

import (
	"context"
	"testing"

	datastore "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestGetChainManager(t *testing.T) {
	assert := assert.New(t)

	ds := datastore.NewMapDatastore()
	cm := getChainManager(ds)
	assert.IsType(&core.ChainManager{}, cm)
}

func TestAddFakeChain(t *testing.T) {
	assert := assert.New(t)

	var length = 9
	var gbbCount, pbCount int

	fake(length, fakeDeps{
		getBestBlock: func() *types.Block {
			gbbCount++
			return new(types.Block)
		},
		processNewBlock: func(context context.Context, block *types.Block) (core.BlockProcessResult, error) {
			pbCount++
			return 0, nil
		},
	})
	assert.Equal(1, gbbCount)
	assert.Equal(length, pbCount)
}
