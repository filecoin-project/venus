package mining

import (
	"context"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/mock"
)

// MockBlockGenerator is a testify mock for BlockGenerator
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
