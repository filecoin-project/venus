package commands

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainRun(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	cst := hamt.NewCborStore()
	nd := &node.Node{ChainMgr: core.NewChainManager(cst), CborStore: cst}

	// Chain of height two.
	err := nd.ChainMgr.Genesis(ctx, core.InitGenesis)
	assert.NoError(err)
	gen := nd.ChainMgr.GetBestBlock()
	child := &types.Block{Height: 1, Parent: gen.Cid(), StateRoot: gen.StateRoot}
	_, err = nd.ChainMgr.ProcessNewBlock(ctx, child)
	assert.NoError(err)
	env := &Env{node: nd}

	out, err := testhelpers.RunCommand(chainLsCmd, nil, nil, env)
	assert.NoError(err)

	assert.Contains(out.Raw, "1337")
	assert.Contains(out.Raw, "zDPWYqFCyJbt3rimt4hyXtwTg6Dkr3FLUisDXo4hLMjxLpsH5cx5")
}

func TestChainTextEncoder(t *testing.T) {
	assert := assert.New(t)

	var a, b types.Block

	b.Height = 1
	assert.NoError(b.AddParent(a))

	var buf bytes.Buffer
	assert.NoError(chainTextEncoder(nil, &buf, &b))

	// TODO: improve assertions once content is stabilized
	assert.Contains(buf.String(), "zDPWYqFD5TSYHCyrHNXP7jxoL9RpCoQ4EHqQtandav8L1QZmKGDW")
}
