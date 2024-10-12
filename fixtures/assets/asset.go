package assets

import (
	"embed"
	"io"
	"path/filepath"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/klauspost/compress/zstd"
)

//go:embed genesis-car
var carFS embed.FS

func GetGenesis(networkType types.NetworkType) ([]byte, error) {
	fileName := ""
	switch networkType {
	case types.NetworkForce:
		fileName = "forcenet.car.zst"
	case types.NetworkInterop:
		fileName = "interopnet.car.zst"
	case types.NetworkButterfly:
		fileName = "butterflynet.car.zst"
	case types.NetworkCalibnet:
		fileName = "calibnet.car.zst"
	default:
		fileName = "mainnet.car.zst"
	}

	file, err := carFS.Open(filepath.Join("genesis-car", fileName))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder, err := zstd.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer decoder.Close() //nolint

	decompressedBytes, err := io.ReadAll(decoder)
	if err != nil {
		return nil, err
	}

	return decompressedBytes, nil
}

//go:embed proof-params
var paramsFS embed.FS

func GetProofParams() ([]byte, error) {
	return paramsFS.ReadFile(filepath.Join("proof-params", "parameters.json"))
}

func GetSrs() ([]byte, error) {
	return paramsFS.ReadFile(filepath.Join("proof-params", "srs-inner-product.json"))
}
