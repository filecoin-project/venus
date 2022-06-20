package migration

import (
	"os"
	"testing"

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
		repoPath := t.TempDir()
		assert.Nil(t, os.RemoveAll(repoPath))
		t.Log(repoPath)
		assert.Nil(t, repo.InitFSRepo(repoPath, 0, cfg))

		assert.Nil(t, TryToMigrate(repoPath))
		fsRepo, err := repo.OpenFSRepo(repoPath, repo.LatestVersion)
		assert.Nil(t, err)
		newCfg := fsRepo.Config()
		assert.Equal(t, paramsCfg.GenesisNetworkVersion, newCfg.NetworkParams.GenesisNetworkVersion)
		assert.EqualValues(t, paramsCfg.ForkUpgradeParam, newCfg.NetworkParams.ForkUpgradeParam)

	}
}
