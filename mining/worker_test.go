package mining

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestWorker_Mine(t *testing.T) {
	assert := assert.New(t)
	cur := &types.Block{Height: 2}
	next := &types.Block{Height: 3}
	ctx := context.Background()

	// Success
	mockBg, mockStateTree, mockAddNewBlock := &MockBlockGenerator{}, &types.MockStateTree{}, &mockAddNewBlockFunc{}
	w := NewWorker(mockBg, mockAddNewBlock.AddNewBlock)
	mockBg.On("Generate", ctx, cur, mockStateTree).Return(next, nil)
	cid, err := w.Mine(ctx, cur, mockStateTree)
	assert.NoError(err)
	assert.True(cid.Equals(next.Cid()))
	mockBg.AssertExpectations(t)
	mockStateTree.AssertExpectations(t)
	assert.True(mockAddNewBlock.Called)
	assert.Equal(next, mockAddNewBlock.Arg)

	// Block generation fails.
	mockBg, mockStateTree, mockAddNewBlock = &MockBlockGenerator{}, &types.MockStateTree{}, &mockAddNewBlockFunc{}
	w = NewWorker(mockBg, mockAddNewBlock.AddNewBlock)
	mockBg.On("Generate", ctx, cur, mockStateTree).Return(nil, errors.New("boom"))
	cid, err = w.Mine(ctx, cur, mockStateTree)
	assert.Error(err)
	mockBg.AssertExpectations(t)
	mockStateTree.AssertExpectations(t)
}
