package assets

import (
	"embed"
	"path/filepath"

	"github.com/filecoin-project/venus/venus-shared/types"
)

//go:embed genesis-car
var carFS embed.FS

func GetGenesis(networkType types.NetworkType) ([]byte, error) {
	fileName := ""
	switch networkType {
	case types.NetworkForce:
		fileName = "forcenet.car"
	case types.NetworkNerpa:
		fileName = "nerpanet.car"
	case types.NetworkInterop:
		fileName = "interopnet.car"
	case types.NetworkButterfly:
		fileName = "butterflynet.car"
	case types.NetworkCalibnet:
		fileName = "calibnet.car"
	default:
		fileName = "mainnet.car"
	}

	return carFS.ReadFile(filepath.Join("genesis-car", fileName))
}

//go:embed proof-params
var paramsFS embed.FS

func GetProofParams() ([]byte, error) {
	return paramsFS.ReadFile(filepath.Join("proof-params", "parameters.json"))
}

func GetSrs() ([]byte, error) {
	return paramsFS.ReadFile(filepath.Join("proof-params", "srs-inner-product.json"))
}
