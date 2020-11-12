package fork

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/internal/pkg/block"
)

var _ = IFork((*MockFork)(nil))

type MockFork struct{}

func NewMockFork() *MockFork {
	return &MockFork{}
}

func (mockFork *MockFork) HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *block.TipSet) (cid.Cid, error) {
	return root, nil
}

func (sm *MockFork) GetNtwkVersion(ctx context.Context, height abi.ChainEpoch) network.Version {
	return network.Version0
}
