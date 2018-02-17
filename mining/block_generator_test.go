package mining

import (
	"context"
	"errors"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFuncs is a poor man's mock use to stub out the processBlock and flushTree
// functions when testing Generate. Could have used testify mocks for flushTree
// but then I would've had to introduce an interface for tree, unclear whether
// that's warranted yet. This is pretty easy in any case.
type mockFuncs struct {
	Called bool
	Cid    *cid.Cid
}

func (m *mockFuncs) successfulProcessBlockFunc(context.Context, *types.Block) error {
	m.Called = true
	return nil
}

func (m *mockFuncs) failingProcessBlockFunc(context.Context, *types.Block) error {
	m.Called = true
	return errors.New("boom")
}

func (m *mockFuncs) successfulFlushTreeFunc(context.Context) (*cid.Cid, error) {
	m.Called = true
	return m.Cid, nil
}

func (m *mockFuncs) failingFlushTreeFunc(context.Context) (*cid.Cid, error) {
	m.Called = true
	return nil, errors.New("boom")
}

// TODO (fritz) Do something about the test duplication w/AddParent.
func TestBlockGenerator_Generate(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	pool := core.NewMessagePool()
	g := BlockGenerator{pool}
	parent := types.Block{
		Parent: types.SomeCid(),
		Height: uint64(100),
	}

	// With no messages.
	m1, m2 := new(mockFuncs), new(mockFuncs)
	m2.Cid = newCid()
	b, err := g.Generate(context.Background(), &parent, m1.successfulProcessBlockFunc, m2.successfulFlushTreeFunc)
	assert.NoError(t, err)
	assert.Equal(t, parent.Cid(), b.Parent)
	assert.Equal(t, m2.Cid, b.StateRoot)
	assert.Len(t, b.Messages, 0)
	assert.True(t, m1.Called)
	assert.True(t, m2.Called)

	// With messages.
	newMsg := types.NewMessageForTestGetter()
	pool.Add(newMsg())
	pool.Add(newMsg())
	expectedMsgs := 2
	require.Len(t, pool.Pending(), expectedMsgs)
	b, err = g.Generate(context.Background(), &parent, m1.successfulProcessBlockFunc, m2.successfulFlushTreeFunc)
	assert.NoError(t, err)
	assert.Len(t, pool.Pending(), expectedMsgs) // Does not remove them.
	assert.Len(t, b.Messages, expectedMsgs)

	// processBlock fails.
	m1, m2 = new(mockFuncs), new(mockFuncs)
	b, err = g.Generate(context.Background(), &parent, m1.failingProcessBlockFunc, m2.successfulFlushTreeFunc)
	assert.Error(t, err)
	assert.True(t, m1.Called)
	assert.False(t, m2.Called)

	// flushTree fails.
	m1, m2 = new(mockFuncs), new(mockFuncs)
	b, err = g.Generate(context.Background(), &parent, m1.successfulProcessBlockFunc, m2.failingFlushTreeFunc)
	assert.Error(t, err)
	assert.True(t, m1.Called)
	assert.True(t, m2.Called)
}
