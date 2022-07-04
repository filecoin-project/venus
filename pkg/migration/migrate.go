package migration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"

	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/venus-shared/types"
	logging "github.com/ipfs/go-log/v2"
)

var migrateLog = logging.Logger("data_migrate")

type UpgradeFunc func(string) error

type versionInfo struct {
	version uint
	upgrade UpgradeFunc
}

var versionMap = []versionInfo{
	{version: 3, upgrade: Version3Upgrade},
	{version: 4, upgrade: Version4Upgrade},
	{version: 5, upgrade: Version5Upgrade},
	{version: 6, upgrade: Version6Upgrade},
	{version: 7, upgrade: Version7Upgrade},
	{version: 8, upgrade: Version8Upgrade},
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
			migrateLog.Infof("success to upgrade version %d to version %d", localVersion, up.version)
			localVersion = up.version
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
	case types.NetworkMainnet:
		fallthrough
	case types.Network2k:
		fallthrough
	case types.NetworkCalibnet:
		fallthrough
	case types.NetworkNerpa:
		fallthrough
	case types.NetworkInterop:
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
	case types.NetworkMainnet:
		cfg.NetworkParams.ForkUpgradeParam = config.DefaultForkUpgradeParam
	case types.Network2k:
		cfg.NetworkParams.ForkUpgradeParam = networks.Net2k().Network.ForkUpgradeParam
	case types.NetworkCalibnet:
		cfg.NetworkParams.ForkUpgradeParam = networks.Calibration().Network.ForkUpgradeParam
	case types.NetworkForce:
		cfg.NetworkParams.ForkUpgradeParam = networks.ForceNet().Network.ForkUpgradeParam
	case types.NetworkButterfly:
		cfg.NetworkParams.ForkUpgradeParam = networks.ButterflySnapNet().Network.ForkUpgradeParam
	case types.NetworkInterop:
		cfg.NetworkParams.ForkUpgradeParam = networks.InteropNet().Network.ForkUpgradeParam
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

//Version5Upgrade
func Version5Upgrade(repoPath string) (err error) {
	var fsrRepo repo.Repo
	if fsrRepo, err = repo.OpenFSRepo(repoPath, 4); err != nil {
		return
	}
	cfg := fsrRepo.Config()
	switch cfg.NetworkParams.NetworkType {
	case types.NetworkMainnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = 1231620
	case types.Network2k:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = -17
	case types.NetworkCalibnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = 312746
	case types.NetworkForce:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = math.MaxInt32
	case types.NetworkInterop:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = -17
	default:
		return fsrRepo.Close()
	}

	if err = fsrRepo.ReplaceConfig(cfg); err != nil {
		return
	}

	if err = fsrRepo.Close(); err != nil {
		return
	}

	return repo.WriteVersion(repoPath, 5)
}

//Version6Upgrade
func Version6Upgrade(repoPath string) (err error) {
	var fsrRepo repo.Repo
	if fsrRepo, err = repo.OpenFSRepo(repoPath, 5); err != nil {
		return
	}
	cfg := fsrRepo.Config()
	switch cfg.NetworkParams.NetworkType {
	case types.NetworkMainnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = 1231620
	case types.Network2k:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = -17
	case types.NetworkCalibnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = 312746
	case types.NetworkForce:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = -17
	case types.NetworkInterop:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version14
		cfg.NetworkParams.ForkUpgradeParam.UpgradeChocolateHeight = -17
	default:
		return fsrRepo.Close()
	}

	if err = fsrRepo.ReplaceConfig(cfg); err != nil {
		return
	}

	if err = fsrRepo.Close(); err != nil {
		return
	}

	return repo.WriteVersion(repoPath, 6)
}

//Version7Upgrade
func Version7Upgrade(repoPath string) (err error) {
	var fsrRepo repo.Repo
	if fsrRepo, err = repo.OpenFSRepo(repoPath, 6); err != nil {
		return
	}
	cfg := fsrRepo.Config()
	switch cfg.NetworkParams.NetworkType {
	case types.NetworkMainnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight = 1594680
	case types.Network2k:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight = -18
	case types.NetworkCalibnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight = 682006
	case types.NetworkButterfly:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version14
		cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight = -18
	case types.NetworkForce:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight = -18
	case types.NetworkInterop:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeOhSnapHeight = -18
	default:
		return fsrRepo.Close()
	}

	// In order to migrate maxfee
	type MpoolCfg struct {
		MaxFee float64 `json:"maxFee"`
	}
	type tempCfg struct {
		Mpool *MpoolCfg `json:"mpool"`
	}
	data, err := ioutil.ReadFile(filepath.Join(repoPath, "config.json"))
	if err != nil {
		migrateLog.Errorf("open config file failed: %v", err)
	} else {
		// If maxFee value is String(10 FIL), unmarshal failure is expected
		// If maxFee value is Number(10000000000000000000), need convert to FIL(10 FIL)
		tmpCfg := tempCfg{}
		if err := json.Unmarshal(data, &tmpCfg); err == nil {
			maxFee := types.MustParseFIL(fmt.Sprintf("%fattofil", tmpCfg.Mpool.MaxFee))
			cfg.Mpool.MaxFee = maxFee
			migrateLog.Info("convert mpool.maxFee from %v to %s", tmpCfg.Mpool.MaxFee, maxFee.String())
		}
	}

	if err = fsrRepo.ReplaceConfig(cfg); err != nil {
		return
	}

	if err = fsrRepo.Close(); err != nil {
		return
	}

	return repo.WriteVersion(repoPath, 7)
}

//Version8Upgrade
func Version8Upgrade(repoPath string) (err error) {
	var fsrRepo repo.Repo
	if fsrRepo, err = repo.OpenFSRepo(repoPath, 7); err != nil {
		return
	}
	cfg := fsrRepo.Config()
	switch cfg.NetworkParams.NetworkType {
	case types.NetworkMainnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight = 1960320
	case types.Network2k:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version16
		cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight = -19
	case types.NetworkCalibnet:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version0
		cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight = 1044660
	case types.NetworkForce:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version16
		cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight = -19
	case types.NetworkInterop:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version15
		cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight = 100
	case types.NetworkButterfly:
		cfg.NetworkParams.GenesisNetworkVersion = network.Version15
		cfg.NetworkParams.ForkUpgradeParam.UpgradeSkyrHeight = 50
	default:
		return fsrRepo.Close()
	}

	if err = fsrRepo.ReplaceConfig(cfg); err != nil {
		return
	}

	if err = fsrRepo.Close(); err != nil {
		return
	}

	return repo.WriteVersion(repoPath, 8)
}
