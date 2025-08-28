package networks

import (
	"fmt"
	"strings"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("network-params")

func GetNetworkFromName(name string) (types.NetworkType, error) {
	if name == "2k" || strings.HasPrefix(name, "localnet-") {
		return types.Network2k, nil
	}
	if name == "force" {
		return types.NetworkForce, nil
	}
	nt, ok := utils.NetworkNameWithNetworkType[types.NetworkName(name)]
	if !ok {
		return types.NetworkDefault, fmt.Errorf("unknown network name %s", name)
	}
	return nt, nil
}

func SetConfigFromOptions(cfg *config.Config, networkName string) error {
	netcfg, err := GetNetworkConfigFromName(networkName)
	if err != nil {
		return err
	}
	cfg.NetworkParams = &netcfg.Network
	cfg.Bootstrap.Period = netcfg.Bootstrap.Period
	cfg.Bootstrap.AddPeers(netcfg.Bootstrap.Addresses...)
	return nil
}

func SetConfigFromNetworkType(cfg *config.Config, networkType types.NetworkType) error {
	netcfg, err := GetNetworkConfigFromType(networkType)
	if err != nil {
		return err
	}
	oldAllowableClockDriftSecs := cfg.NetworkParams.AllowableClockDriftSecs
	cfg.NetworkParams = &netcfg.Network
	// not change, expect to adjust the value through the configuration file
	cfg.NetworkParams.AllowableClockDriftSecs = oldAllowableClockDriftSecs

	if constants.DisableF3 {
		cfg.NetworkParams.F3Enabled = false
		log.Warn("F3 is disabled")
	}
	return nil
}

func GetNetworkConfigFromType(networkType types.NetworkType) (*NetworkConf, error) {
	switch networkType {
	case types.NetworkMainnet:
		return Mainnet(), nil
	case types.NetworkForce:
		return ForceNet(), nil
	case types.Integrationnet:
		return IntegrationNet(), nil
	case types.Network2k:
		return Net2k(), nil
	case types.NetworkCalibnet:
		return Calibration(), nil
	case types.NetworkInterop:
		return InteropNet(), nil
	case types.NetworkButterfly:
		return ButterflySnapNet(), nil
	}

	return nil, fmt.Errorf("unknown network type %d", networkType)
}

func GetNetworkConfigFromName(networkName string) (*NetworkConf, error) {
	networkType, err := GetNetworkFromName(networkName)
	if err != nil {
		return nil, err
	}

	return GetNetworkConfigFromType(networkType)
}
