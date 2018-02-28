package mining

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockProcessBlock struct {
	mock.Mock
}

func (mpb *MockProcessBlock) ProcessBlock(ctx context.Context, b *types.Block, st types.StateTree) (receipts []*types.MessageReceipt, err error) {
	args := mpb.Called(ctx, b, st)
	if args.Get(0) != nil {
		receipts = args.Get(0).([]*types.MessageReceipt)
	}
	err = args.Error(1)
	return
}

// TODO (fritz) Do something about the test duplication w/AddParent.
func TestBlockGenerator_Generate(t *testing.T) {
	oldProcessBlock := processBlock
	defer func() { processBlock = oldProcessBlock }()
	assert := assert.New(t)
	newCid := types.NewCidForTestGetter()
	pool := core.NewMessagePool()
	g := blockGenerator{pool}
	parent := types.Block{
		Parent: types.SomeCid(),
		Height: uint64(100),
	}

	// With no messages.
	expectedCid := newCid()
	mpb, mst := &MockProcessBlock{}, &types.MockStateTree{}
	processBlock = mpb.ProcessBlock
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return([]*types.MessageReceipt{}, nil)
	mst.On("Flush", context.Background()).Return(expectedCid, nil)
	b, err := g.Generate(context.Background(), &parent, mst)
	assert.NoError(err)
	assert.Equal(parent.Cid(), b.Parent)
	assert.Equal(expectedCid, b.StateRoot)
	assert.Len(b.Messages, 0)
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// With messages.
	expectedCid = newCid()
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	processBlock = mpb.ProcessBlock
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return([]*types.MessageReceipt{}, nil)
	mst.On("Flush", context.Background()).Return(expectedCid, nil)
	newMsg := types.NewMessageForTestGetter()
	pool.Add(newMsg())
	pool.Add(newMsg())
	expectedMsgs := 2
	require.Len(t, pool.Pending(), expectedMsgs)
	b, err = g.Generate(context.Background(), &parent, mst)
	assert.NoError(err)
	assert.Len(pool.Pending(), expectedMsgs) // Does not remove them.
	assert.Len(b.Messages, expectedMsgs)
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// processBlock fails.
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	processBlock = mpb.ProcessBlock
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return(nil, errors.New("boom ProcessBlock failed"))
	_, err = g.Generate(context.Background(), &parent, mst)
	if assert.Error(err) {
		assert.Contains(err.Error(), "ProcessBlock")
	}
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// tree.Flush fails.
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	processBlock = mpb.ProcessBlock
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return([]*types.MessageReceipt{}, nil)
	mst.On("Flush", context.Background()).Return(nil, errors.New("boom tree.Flush failed"))
	_, err = g.Generate(context.Background(), &parent, mst)
	if assert.Error(err) {
		assert.Contains(err.Error(), "Flush")
	}
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)
}
