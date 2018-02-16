package mining

import (
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: we should put this in a test helper somewhere so we can just get a cid when
// we need one in tests and don't care about what it is. Where do we put it?
func testCid() *cid.Cid {
	b := &types.Block{}
	return b.Cid()
}

func TestBlockGenerator_Generate(t *testing.T) {
	pool := core.NewMessagePool()
	g := BlockGenerator{pool}
	parent := types.Block{
		Parent: testCid(),
		Height: uint64(100),
	}

	// With no messages.
	b, err := g.Generate(&parent)
	assert.NoError(t, err)
	assert.Equal(t, parent.Cid(), b.Parent)
	assert.Len(t, b.Messages, 0)

	// With messages.
	newMsg := types.NewMessageForTestGetter()
	pool.Add(newMsg())
	pool.Add(newMsg())
	expectedMsgs := 2
	require.Len(t, pool.Pending(), expectedMsgs)
	b, err = g.Generate(&parent)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), expectedMsgs) // Does not remove them.
	assert.Len(t, b.Messages, expectedMsgs)
}
