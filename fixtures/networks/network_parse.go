package networks

import (
	"fmt"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
)

func GetNetworkFromName(name string) (types.NetworkType, error) {
	if name == "2k" {
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
	netcfg, err := GetNetworkConfig(networkName)
	if err != nil {
		return err
	}
	cfg.Bootstrap = &netcfg.Bootstrap
	cfg.NetworkParams = &netcfg.Network
	return nil
}

func SetConfigFromNetworkType(cfg *config.Config, networkType types.NetworkType) error {
	netcfg, err := GetNetworkConfig(networkType)
	if err != nil {
		return err
	}
	cfg.NetworkParams = &netcfg.Network
	return nil
}

func GetNetworkConfig(network interface{}) (*NetworkConf, error) {
	var networkType types.NetworkType
	var err error

	switch val := network.(type) {
	case types.NetworkType:
		networkType = val
	case string:
		networkType, err = GetNetworkFromName(val)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("not expect type %T %v", network, network)
	}

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
	return nil, fmt.Errorf("unknown network name %s", network)
}
