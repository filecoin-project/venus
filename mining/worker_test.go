package mining

import (
	"context"
	"errors"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockBlockGenerator struct {
	mock.Mock
}

func (bg *MockBlockGenerator) Generate(ctx context.Context, h *types.Block, pbf ProcessBlockFunc, ftf FlushTreeFunc) (b *types.Block, err error) {
	args := bg.Called(ctx, h, pbf, ftf)
	if args.Get(0) != nil {
		b = args.Get(0).(*types.Block)
	}
	err = args.Error(1)
	return
}

type mockAddNewBlockFunc struct {
	Called bool
	Arg    *types.Block
}

func (m *mockAddNewBlockFunc) AddNewBlock(ctx context.Context, b *types.Block) error {
	m.Called = true
	m.Arg = b
	return nil
}

// Implements types.StateTreeInterface
type FakeStateTree struct{}

func (f FakeStateTree) Flush(ctx context.Context) (*cid.Cid, error) {
	return types.SomeCid(), nil
}
func (f FakeStateTree) GetActor(context.Context, types.Address) (*types.Actor, error) {
	panic("boom -- not expected to be called")
	return nil, nil
}
func (f FakeStateTree) SetActor(context.Context, types.Address, *types.Actor) error {
	panic("boom -- not expected to be called")
	return nil
}

func TestWorker_Mine(t *testing.T) {
	assert := assert.New(t)
	cur := &types.Block{Height: 2}
	next := &types.Block{Height: 3}
	ctx := context.Background()

	// Success
	mockBg, mockAddNewBlock := &MockBlockGenerator{}, &mockAddNewBlockFunc{}
	w := NewWorker(mockBg, mockAddNewBlock.AddNewBlock)
	mockBg.On("Generate", ctx, cur, mock.AnythingOfType("ProcessBlockFunc"), mock.AnythingOfType("FlushTreeFunc")).Return(next, nil)
	cid, err := w.Mine(ctx, cur, FakeStateTree{})
	assert.NoError(err)
	assert.True(cid.Equals(next.Cid()))
	mockBg.AssertExpectations(t)
	assert.True(mockAddNewBlock.Called)
	assert.Equal(next, mockAddNewBlock.Arg)

	// Block generation fails.
	mockBg, mockAddNewBlock = &MockBlockGenerator{}, &mockAddNewBlockFunc{}
	w = NewWorker(mockBg, mockAddNewBlock.AddNewBlock)
	mockBg.On("Generate", ctx, cur, mock.AnythingOfType("ProcessBlockFunc"), mock.AnythingOfType("FlushTreeFunc")).Return(nil, errors.New("boom"))
	cid, err = w.Mine(ctx, cur, FakeStateTree{})
	assert.Error(err)
	mockBg.AssertExpectations(t)
}
