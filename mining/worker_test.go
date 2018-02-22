package mining

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestWorker_Start(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	baseBlock := &types.Block{}
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

	_ = <-NewWorker(baseBlock, mockBg, mockStateTree).Start(ctx)
	assert.True(mineCalled)
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
