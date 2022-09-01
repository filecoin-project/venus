package utils

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var NetworkNameWithNetworkType = map[types.NetworkName]types.NetworkType{
	types.NetworkNameMain:        types.NetworkMainnet,
	types.NetworkNameCalibration: types.NetworkCalibnet,
	types.NetworkNameButterfly:   types.NetworkButterfly,
	types.NetworkNameInterop:     types.NetworkInterop,
	types.NetworkNameIntegration: types.Integrationnet,
}

var NetworkTypeWithNetworkName = func() map[types.NetworkType]types.NetworkName {
	typeName := make(map[types.NetworkType]types.NetworkName, len(NetworkNameWithNetworkType))
	for nt, nn := range NetworkNameWithNetworkType {
		typeName[nn] = nt
	}

	return typeName
}()

func NetworkNameToNetworkType(networkName types.NetworkName) (types.NetworkType, error) {
	if len(networkName) == 0 {
		return types.NetworkDefault, fmt.Errorf("network name is empty")
	}
	nt, ok := NetworkNameWithNetworkType[networkName]
	if ok {
		return nt, nil
	}
	// 2k and force networks do not have exact network names
	return types.Network2k, nil
}

func NetworkTypeToNetworkName(networkType types.NetworkType) types.NetworkName {
	nn, ok := NetworkTypeWithNetworkName[networkType]
	if ok {
		return nn
	}

	// 2k and force networks do not have exact network names
	return ""
}

type networkNameGetter interface {
	StateNetworkName(ctx context.Context) (types.NetworkName, error)
}

func LoadBuiltinActors(ctx context.Context, getter networkNameGetter) error {
	networkName, err := getter.StateNetworkName(ctx)
	if err != nil {
		return err
	}
	nt, err := NetworkNameToNetworkType(networkName)
	if err != nil {
		return err
	}
	if err := actors.SetNetworkBundle(int(nt)); err != nil {
		return err
	}
	ReloadMethodsMap()

	return nil
}
