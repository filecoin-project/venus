package commands

import (
	"context"
	"testing"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"

	"github.com/stretchr/testify/assert"

	"encoding/json"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainHead(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	nd := CreateInMemoryOfflineTestCmdHarness().node
	env := &Env{node: nd}

	err := nd.ChainMgr.Genesis(ctx, core.InitGenesis)
	assert.NoError(err)

	gen := nd.ChainMgr.GetBestBlock()

	out, err := testhelpers.RunCommand(chainHeadCmd, nil, map[string]interface{}{
		cmds.EncLong: cmds.JSON,
	}, env)

	assert.NoError(err)
	assert.Contains(out.Raw, gen.Cid().String())
}

func TestChainLsRun(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	h := CreateInMemoryOfflineTestCmdHarness()

	// Chain of height two.
	err := h.chainmanager.Genesis(ctx, core.InitGenesis)
	assert.NoError(err)
	gen := h.chainmanager.GetBestBlock()
	child := &types.Block{Height: 1, Parent: gen.Cid(), StateRoot: gen.StateRoot}
	_, err = h.chainmanager.ProcessNewBlock(ctx, child)
	assert.NoError(err)
	env := &Env{node: h.node}

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
