package migration

import (
	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/repo"
	logging "github.com/ipfs/go-log/v2"
)

var migrateLog = logging.Logger("migrate")

type UpgradeFunc func(string) error

type versionInfo struct {
	version uint
	upgrade UpgradeFunc
}

var versionMap = []versionInfo{
	{version: 3, upgrade: Version3Upgrade},
	{version: 4, upgrade: Version4Upgrade},
}

// TryToMigrate used to migrate data(db,config,file,etc) in local repo
func TryToMigrate(repoPath string) error {
	localVersion, err := repo.ReadVersion(repoPath)
	if err != nil {
		return err
	}

	for _, up := range versionMap {
		if up.version > localVersion {
			err = up.upgrade(repoPath)
			if err != nil {
				return err
			}
			localVersion = up.version
			migrateLog.Infof("success to upgrade version %d to version %d", localVersion, up.version)
		}
	}

	return nil
}

// Version3Upgrade 3 for a config filed named apiAuthUrl
func Version3Upgrade(repoPath string) error {
	fsrRepo, err := repo.OpenFSRepo(repoPath, 2)
	if err != nil {
		return err
	}

	cfg := fsrRepo.Config()

	switch cfg.NetworkParams.NetworkType {
	case constants.NetworkMainnet:
		fallthrough
	case constants.Network2k:
		fallthrough
	case constants.NetworkCalibnet:
		fallthrough
	case constants.NetworkNerpa:
		fallthrough
	case constants.NetworkInterop:
		cfg.API.VenusAuthURL = ""
	}

	err = fsrRepo.ReplaceConfig(cfg)
	if err != nil {
		return err
	}
	err = fsrRepo.Close()
	if err != nil {
		return err
	}
	return repo.WriteVersion(repoPath, 3)
}

func Version4Upgrade(repoPath string) (err error) {
	var fsrRepo repo.Repo
	if fsrRepo, err = repo.OpenFSRepo(repoPath, 3); err != nil {
		return
	}
	cfg := fsrRepo.Config()
	switch cfg.NetworkParams.NetworkType {
	case constants.NetworkMainnet:
		cfg.NetworkParams.ForkUpgradeParam = config.DefaultForkUpgradeParam
	case constants.Network2k:
		cfg.NetworkParams.ForkUpgradeParam = networks.Net2k().Network.ForkUpgradeParam
	case constants.NetworkCalibnet:
		cfg.NetworkParams.ForkUpgradeParam = networks.Calibration().Network.ForkUpgradeParam
	case constants.NetworkNerpa:
		cfg.NetworkParams.ForkUpgradeParam = networks.NerpaNet().Network.ForkUpgradeParam
	case constants.NetworkForce:
		cfg.NetworkParams.ForkUpgradeParam = networks.ForceNet().Network.ForkUpgradeParam
	default:
		return fsrRepo.Close()
	}

	if err = fsrRepo.ReplaceConfig(cfg); err != nil {
		return
	}

	if err = fsrRepo.Close(); err != nil {
		return
	}

	return repo.WriteVersion(repoPath, 4)
}
