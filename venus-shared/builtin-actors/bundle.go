package builtinactors

import (
	"bytes"
	_ "embed"

	"github.com/BurntSushi/toml"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var NetworkBundle string

func SetNetworkBundle(networkType types.NetworkType) {
	switch networkType {
	case types.Network2k, types.NetworkForce:
		NetworkBundle = "devnet"
	case types.NetworkButterfly:
		NetworkBundle = "butterflynet"
	case types.NetworkInterop:
		NetworkBundle = "caterpillarnet"
	case types.NetworkCalibnet:
		NetworkBundle = "calibrationnet"
	case types.NetworkMainnet:
		NetworkBundle = "mainnet"
	}
}

//go:embed bundles.toml
var BuiltinActorBundles []byte

type BundleSpec struct {
	Bundles []Bundle
}

type Bundle struct {
	Version actors.Version
	Release string
}

var BuiltinActorReleases map[actors.Version]string

func init() {
	BuiltinActorReleases = make(map[actors.Version]string)

	spec := BundleSpec{}

	r := bytes.NewReader(BuiltinActorBundles)
	_, err := toml.DecodeReader(r, &spec)
	if err != nil {
		panic(err)
	}

	for _, b := range spec.Bundles {
		BuiltinActorReleases[b.Version] = b.Release
	}
}
