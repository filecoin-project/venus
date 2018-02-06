package commands

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainRun(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	cst := hamt.NewCborStore()
	nd := &node.Node{ChainMgr: chain.NewChainManager(cst), CborStore: cst}

	// Chain of height two.
	h := uint64(43)
	child := &types.Block{Height: h}
	parent := &types.Block{Height: h - 1}
	child.AddParent(*parent)

	_, err := cst.Put(ctx, child)
	assert.NoError(err)
	_, err = cst.Put(ctx, parent)
	assert.NoError(err)

	assert.NoError(nd.ChainMgr.SetBestBlock(ctx, child))
	env := &Env{node: nd}

	out, err := testhelpers.RunCommand(chainLsCmd, []string{"chain", "ls"}, env)
	assert.NoError(err)

	assert.Contains(out, fmt.Sprintf("%d", child.Height))
	assert.Contains(out, fmt.Sprintf("%d", parent.Height))
}

func TestChainTextEncoder(t *testing.T) {
	assert := assert.New(t)

	var a, b types.Block

	b.Height = 1
	assert.NoError(b.AddParent(a))

	var buf bytes.Buffer
	assert.NoError(chainTextEncoder(nil, &buf, &b))

	// TODO: improve assertions once content is stabilized
	assert.Contains(buf.String(), "zDPWYqFD7dM75R3hLV92mVNPjRX1dxiyUxuhKwBgVns5FRPC4Vak")
}
