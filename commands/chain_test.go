package commands

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainRun(t *testing.T) {
	assert := assert.New(t)

	// No node.
	env := Env{node: nil}
	_, err := testhelpers.RunCommand(chainCmd, []string{"chain"}, &env)
	assert.NoError(err)

	// No block.
	env = Env{node: &node.Node{Block: nil}}
	_, err = testhelpers.RunCommand(chainCmd, []string{"chain"}, &env)
	assert.NoError(err)

	// Chain of height two.
	h := uint64(43)
	child := &types.Block{Height: h}
	parent := &types.Block{Height: h - 1}
	child.AddParent(*parent)
	n := node.Node{Block: child}
	env = Env{node: &n}

	out, err := testhelpers.RunCommand(chainCmd, []string{"chain"}, &env)
	assert.NoError(err)

	assert.Contains(out, fmt.Sprintf("%d", child.Height))
	// TODO enable this test when we can walk the chain.
	// assert.Contains(out, fmt.Sprintf("%d", parent.Height))
}

func TestChainTextEncoder(t *testing.T) {
	assert := assert.New(t)

	var a, b types.Block

	b.Height = 1
	assert.NoError(b.AddParent(a))

	var buf bytes.Buffer
	assert.NoError(chainTextEncoder(nil, &buf, &b))

	// TODO: improve assertions once content is stabilized
	assert.Contains(buf.String(), "zDPWYqFD4pM9w2M4hbH4rAxKSmwQ3hdEYm4ohGaBXLEfCuTDGsK1")
}
