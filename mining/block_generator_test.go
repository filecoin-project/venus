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

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
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
	assert := assert.New(t)
	newCid := types.NewCidForTestGetter()
	pool := core.NewMessagePool()
	baseBlock := types.Block{
		Parent:    newCid(),
		Height:    uint64(100),
		StateRoot: newCid(),
	}

	// With no messages.
	nextStateRoot := newCid()
	mpb, mst := &MockProcessBlock{}, &types.MockStateTree{}
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return([]*types.MessageReceipt{}, nil)
	mst.On("Flush", context.Background()).Return(nextStateRoot, nil)
	successfulGetStateTree := func(c context.Context, stateRootCid *cid.Cid) (types.StateTree, error) {
		assert.True(stateRootCid.Equals(baseBlock.StateRoot))
		return mst, nil
	}
	g := NewBlockGenerator(pool, successfulGetStateTree, mpb.ProcessBlock)
	next, err := g.Generate(context.Background(), &baseBlock)
	assert.NoError(err)
	assert.Equal(baseBlock.Cid(), next.Parent)
	assert.Equal(nextStateRoot, next.StateRoot)
	assert.Len(next.Messages, 0)
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// With messages.
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return([]*types.MessageReceipt{}, nil)
	mst.On("Flush", context.Background()).Return(nextStateRoot, nil)
	newMsg := types.NewMessageForTestGetter()
	pool.Add(newMsg())
	pool.Add(newMsg())
	expectedMsgs := 2
	require.Len(t, pool.Pending(), expectedMsgs)
	g = NewBlockGenerator(pool, successfulGetStateTree, mpb.ProcessBlock)
	next, err = g.Generate(context.Background(), &baseBlock)
	assert.NoError(err)
	assert.Len(pool.Pending(), 2)
	assert.Len(next.Messages, expectedMsgs)
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// getStateTree fails.
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	explodingGetStateTree := func(c context.Context, stateRootCid *cid.Cid) (types.StateTree, error) {
		return nil, errors.New("boom getStateTree failed")
	}
	g = NewBlockGenerator(pool, explodingGetStateTree, mpb.ProcessBlock)
	next, err = g.Generate(context.Background(), &baseBlock)
	if assert.Error(err) {
		assert.Contains(err.Error(), "getStateTree")
	}
	assert.Nil(next)
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// processBlock fails.
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return(nil, errors.New("boom ProcessBlock failed"))
	g = NewBlockGenerator(pool, successfulGetStateTree, mpb.ProcessBlock)
	_, err = g.Generate(context.Background(), &baseBlock)
	if assert.Error(err) {
		assert.Contains(err.Error(), "ProcessBlock")
	}
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)

	// tree.Flush fails.
	mpb, mst = &MockProcessBlock{}, &types.MockStateTree{}
	mpb.On("ProcessBlock", context.Background(), mock.AnythingOfType("*types.Block"), mst).Return([]*types.MessageReceipt{}, nil)
	mst.On("Flush", context.Background()).Return(nil, errors.New("boom tree.Flush failed"))
	g = NewBlockGenerator(pool, successfulGetStateTree, mpb.ProcessBlock)
	_, err = g.Generate(context.Background(), &baseBlock)
	if assert.Error(err) {
		assert.Contains(err.Error(), "Flush")
	}
	mpb.AssertExpectations(t)
	mst.AssertExpectations(t)
}
