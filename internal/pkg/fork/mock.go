package fork

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

var _ = IFork((*MockFork)(nil))

type MockFork struct{}

func NewMockFork() *MockFork {
	return &MockFork{}
}

func (mockFork *MockFork) HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	return root, nil
}
