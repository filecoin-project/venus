package utils

import (
	"context"
	"fmt"

	builtinactors "github.com/filecoin-project/venus/venus-shared/builtin-actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var NameType = map[types.NetworkName]types.NetworkType{
	types.NetworkNameMain:        types.NetworkMainnet,
	types.NetworkNameCalibration: types.NetworkCalibnet,
	types.NetworkNameButterfly:   types.NetworkButterfly,
	types.NetworkNameInterop:     types.NetworkInterop,
	types.NetworkNameIntegration: types.Integrationnet,
}

var TypeName = func() map[types.NetworkType]types.NetworkName {
	typeName := make(map[types.NetworkType]types.NetworkName, len(NameType))
	for nt, nn := range NameType {
		typeName[nn] = nt
	}

	return typeName
}()

func NetworkNameToNetworkType(networkName types.NetworkName) (types.NetworkType, error) {
	if len(networkName) == 0 {
		return types.NetworkDefault, fmt.Errorf("network name is empty")
	}
	nt, ok := NameType[networkName]
	if ok {
		return nt, nil
	}
	// 2k and force networks do not have exact network names
	return types.Network2k, nil
}

func NetworkTypeToNetworkName(networkType types.NetworkType) types.NetworkName {
	nn, ok := TypeName[networkType]
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
	if err := builtinactors.SetNetworkBundle(nt); err != nil {
		return err
	}
	ReloadMethodsMap()

	return nil
}
