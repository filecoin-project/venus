package fork

import (
	"context"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
)

var _ = IFork((*MockFork)(nil))

type MockFork struct{}

//NewMockFork mock for test
func NewMockFork() *MockFork {
	return &MockFork{}
}

func (mockFork *MockFork) HandleStateForks(ctx context.Context, root cid.Cid, height abi.ChainEpoch, ts *types.TipSet) (cid.Cid, error) {
	return root, nil
}

func (mockFork *MockFork) GetNtwkVersion(ctx context.Context, height abi.ChainEpoch) network.Version {
	return network.Version0
}

func (mockFork *MockFork) HasExpensiveFork(ctx context.Context, height abi.ChainEpoch) bool {
	return false
}

func (mockFork *MockFork) GetForkUpgrade() *config.ForkUpgradeConfig {
	return &config.ForkUpgradeConfig{
		UpgradeSmokeHeight:       -1,
		UpgradeBreezeHeight:      -1,
		UpgradeIgnitionHeight:    -1,
		UpgradeLiftoffHeight:     -1,
		UpgradeActorsV2Height:    -1,
		UpgradeRefuelHeight:      -1,
		UpgradeTapeHeight:        -1,
		UpgradeKumquatHeight:     -1,
		BreezeGasTampingDuration: -1,
		UpgradeCalicoHeight:      -1,
		UpgradePersianHeight:     -1,
		UpgradeOrangeHeight:      -1,
		UpgradeClausHeight:       -1,
	}
}

func (mockFork *MockFork) Start(ctx context.Context) error {

	return nil
}
