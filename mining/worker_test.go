package mining

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func TestWorker_Start(t *testing.T) {
	newCid := types.NewCidForTestGetter()
	assert := assert.New(t)
	ctx := context.Background()
	baseBlock := &types.Block{StateRoot: newCid()}
	mockBg, mockStateTree := &MockBlockGenerator{}, &types.MockStateTree{}

	var mineCalled bool
	oldMineFunc := mineFunc
	defer func() { mineFunc = oldMineFunc }()
	mineFunc = func(c context.Context, b *types.Block, st types.StateTree, bg BlockGenerator, resCh chan<- Result) {
		assert.Equal(ctx, c)
		assert.Equal(b, baseBlock)
		assert.Equal(mockBg, bg)
		mineCalled = true
		resCh <- Result{}
	}

	// Success.
	worker := NewWorker(mockBg, func(contextIgnored context.Context, cid *cid.Cid) (types.StateTree, error) {
		assert.True(cid.Equals(baseBlock.StateRoot))
		return mockStateTree, nil
	})
	assert.NotNil(worker)
	_ = <-worker.Start(ctx, baseBlock)
	assert.True(mineCalled)
	mockBg.AssertExpectations(t)
	mockStateTree.AssertExpectations(t)

	// Fails to load state tree.
	mineCalled = false
	mockBg, mockStateTree = &MockBlockGenerator{}, &types.MockStateTree{}
	worker = NewWorker(mockBg, func(contextIgnored context.Context, cid *cid.Cid) (types.StateTree, error) {
		return nil, errors.New("boom")
	})
	assert.NotNil(worker)
	result := <-worker.Start(ctx, baseBlock)
	assert.False(mineCalled)
	assert.Error(result.Err)
	assert.Contains(result.Err.Error(), "state tree")
	mockBg.AssertExpectations(t)
	mockStateTree.AssertExpectations(t)
}

func Test_mine(t *testing.T) {
	assert := assert.New(t)
	cur := &types.Block{Height: 2}
	next := &types.Block{Height: 3}
	ctx := context.Background()

	// Success
	mockBg, mockStateTree := &MockBlockGenerator{}, &types.MockStateTree{}
	resCh := make(chan Result)
	mockBg.On("Generate", ctx, cur, mockStateTree).Return(next, nil)
	go mine(ctx, cur, mockStateTree, mockBg, resCh)
	r := <-resCh
	assert.NoError(r.Err)
	assert.True(r.NewBlock.Cid().Equals(next.Cid()))
	mockBg.AssertExpectations(t)
	mockStateTree.AssertExpectations(t)

	// Block generation fails.
	mockBg, mockStateTree = &MockBlockGenerator{}, &types.MockStateTree{}
	resCh = make(chan Result)
	mockBg.On("Generate", ctx, cur, mockStateTree).Return(nil, errors.New("boom"))
	go mine(ctx, cur, mockStateTree, mockBg, resCh)
	r = <-resCh
	assert.Error(r.Err)
	mockBg.AssertExpectations(t)
	mockStateTree.AssertExpectations(t)
}
