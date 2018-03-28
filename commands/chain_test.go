package commands

import (
	"context"
	"testing"

	"gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	"gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	"github.com/stretchr/testify/assert"

	"encoding/json"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainRun(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	cst := hamt.NewCborStore()
	ds := datastore.NewMapDatastore()
	nd := &node.Node{ChainMgr: core.NewChainManager(ds, cst), CborStore: cst}

	// Chain of height two.
	err := nd.ChainMgr.Genesis(ctx, core.InitGenesis)
	assert.NoError(err)
	gen := nd.ChainMgr.GetBestBlock()
	child := &types.Block{Height: 1, Parent: gen.Cid(), StateRoot: gen.StateRoot}
	_, err = nd.ChainMgr.ProcessNewBlock(ctx, child)
	assert.NoError(err)
	env := &Env{node: nd}

	out, err := testhelpers.RunCommand(chainLsCmd, nil, map[string]interface{}{
		cmds.EncLong: cmds.JSON,
	}, env)
	assert.NoError(err)

	results := make([]*types.Block, 2)
	for i, line := range out.Lines {
		if line == "" {
			continue
		}

		var result types.Block
		err = json.Unmarshal([]byte(line), &result)
		assert.NoError(err)

		results[i] = &result
	}

	assert.Equal(len(results), 2)
	assert.Nil(results[1].Parent)
	assert.Equal(gen.Cid(), results[0].Parent)
}
