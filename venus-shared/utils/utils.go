package utils

import (
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/types"
)

var NameType = map[types.NetworkName]types.NetworkType{
	"mainnet":        types.NetworkMainnet,
	"calibrationnet": types.NetworkCalibnet,
	"butterflynet":   types.NetworkButterfly,
	"interopnet":     types.NetworkInterop,
	"integrationnet": types.Integrationnet,
}

var TypeName = map[types.NetworkType]types.NetworkName{
	types.NetworkMainnet:   "mainnet",
	types.NetworkCalibnet:  "calibrationnet",
	types.NetworkButterfly: "butterflynet",
	types.NetworkInterop:   "interopnet",
	types.Integrationnet:   "integrationnet",
}

var ErrMay2kNetwork = fmt.Errorf("may 2k network")

func NetworkNameToNetworkType(networkName types.NetworkName) (types.NetworkType, error) {
	if len(networkName) == 0 {
		return types.NetworkDefault, fmt.Errorf("network name is empty")
	}
	nt, ok := NameType[networkName]
	if ok {
		return nt, nil
	}
	// 2k and force networks do not have exact network names
	return types.NetworkDefault, ErrMay2kNetwork
}
