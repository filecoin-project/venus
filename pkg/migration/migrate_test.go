package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestMigration(t *testing.T) {
	tf.UnitTest(t)

	cfgs := map[types.NetworkType]*config.NetworkParamsConfig{
		types.Network2k:        &networks.Net2k().Network,
		types.NetworkForce:     &networks.ForceNet().Network,
		types.NetworkInterop:   &networks.InteropNet().Network,
		types.NetworkButterfly: &networks.ButterflySnapNet().Network,
		types.NetworkCalibnet:  &networks.Calibration().Network,
		types.NetworkMainnet:   &networks.Mainnet().Network,
	}

	for nt, paramsCfg := range cfgs {
		cfg := config.NewDefaultConfig()
		cfg.NetworkParams.NetworkType = nt
		cfg.NetworkParams.AllowableClockDriftSecs = 10
		repoPath := filepath.Join(os.TempDir(), fmt.Sprintf("TestMigration%d", time.Now().UnixNano()))

		assert.Nil(t, repo.InitFSRepo(repoPath, 0, cfg))

		assert.Nil(t, TryToMigrate(repoPath))
		fsRepo, err := repo.OpenFSRepo(repoPath, repo.LatestVersion)
		assert.Nil(t, err)
		newCfg := fsRepo.Config()
		errStr := fmt.Sprintf("current network type %d", paramsCfg.NetworkType)
		assert.Equalf(t, paramsCfg.NetworkType, newCfg.NetworkParams.NetworkType, errStr)
		assert.Equalf(t, uint64(0), newCfg.NetworkParams.BlockDelay, errStr)
		assert.Equalf(t, paramsCfg.AllowableClockDriftSecs, newCfg.NetworkParams.AllowableClockDriftSecs, errStr)
		assert.EqualValuesf(t, config.NewDefaultConfig().NetworkParams.ForkUpgradeParam, newCfg.NetworkParams.ForkUpgradeParam, errStr)
		assert.NoError(t, fsRepo.Close())
	}
}
