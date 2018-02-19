package mining

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWorker_Mine(t *testing.T) {
	assert := assert.New(t)
	cur := &types.Block{Height: 2}
	next := &types.Block{Height: 3}
	ctx := context.Background()

	// Success
	mockBg, mockAddNewBlock := &MockBlockGenerator{}, &mockAddNewBlockFunc{}
	w := NewWorker(mockBg, mockAddNewBlock.AddNewBlock)
	mockBg.On("Generate", ctx, cur, mock.AnythingOfType("ProcessBlockFunc"), mock.AnythingOfType("FlushTreeFunc")).Return(next, nil)
	cid, err := w.Mine(ctx, cur, types.FakeStateTree{})
	assert.NoError(err)
	assert.True(cid.Equals(next.Cid()))
	mockBg.AssertExpectations(t)
	assert.True(mockAddNewBlock.Called)
	assert.Equal(next, mockAddNewBlock.Arg)

	// Block generation fails.
	mockBg, mockAddNewBlock = &MockBlockGenerator{}, &mockAddNewBlockFunc{}
	w = NewWorker(mockBg, mockAddNewBlock.AddNewBlock)
	mockBg.On("Generate", ctx, cur, mock.AnythingOfType("ProcessBlockFunc"), mock.AnythingOfType("FlushTreeFunc")).Return(nil, errors.New("boom"))
	cid, err = w.Mine(ctx, cur, types.FakeStateTree{})
	assert.Error(err)
	mockBg.AssertExpectations(t)
}
