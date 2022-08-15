package networks

import (
	"fmt"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func GetNetworkFromName(name string) (types.NetworkType, error) {
	switch name {
	case string(types.NetworkNameMain):
		return types.NetworkMainnet, nil
	case "force":
		return types.NetworkForce, nil
	case string(types.NetworkNameIntegration):
		return types.Integrationnet, nil
	case "2k":
		return types.Network2k, nil
	case "cali", string(types.NetworkNameCalibration):
		return types.NetworkCalibnet, nil
	case "interop", string(types.NetworkNameInterop):
		return types.NetworkInterop, nil
	case "butterfly", string(types.NetworkNameButterfly):
		return types.NetworkButterfly, nil
	default:
		return 0, fmt.Errorf("unknown network name %s", name)
	}
}

func SetConfigFromOptions(cfg *config.Config, network string) error {
	netcfg, err := GetNetworkConfig(network)
	if err != nil {
		return err
	}
	if netcfg != nil {
		cfg.Bootstrap = &netcfg.Bootstrap
		cfg.NetworkParams = &netcfg.Network
	}
	return nil
}

func GetNetworkConfig(network string) (*NetworkConf, error) {
	networkType, err := GetNetworkFromName(network)
	if err != nil {
		return nil, err
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
