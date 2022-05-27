package builtinactors

import (
	"bytes"
	_ "embed"

	"github.com/BurntSushi/toml"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var NetworkBundle string
var ActorsCIDs = make(map[actors.Version]cid.Cid)

func SetNetworkBundle(networkType types.NetworkType) {
	switch networkType {
	case types.Network2k, types.NetworkForce:
		NetworkBundle = "devnet"
	case types.NetworkButterfly:
		NetworkBundle = "butterflynet"
	case types.NetworkInterop:
		NetworkBundle = "caterpillarnet"
		ActorsCIDs[actors.Version8] = types.MustParseCid("bafy2bzaceadr77tamp35bbb3rtio4ver4pnk2cbxqif3nn3mrmxra2nlvwoce")
	case types.NetworkCalibnet:
		NetworkBundle = "calibrationnet"
	default:
		NetworkBundle = "mainnet"
	}
}

// Must have called SetNetworkBundle before calling GetActorsCIDs
func GetActorsCIDs() map[actors.Version]cid.Cid {
	return ActorsCIDs
}

//go:embed bundles.toml
var BuiltinActorBundles []byte

type BundleSpec struct {
	Bundles []Bundle
}

type Bundle struct {
	// Version is the actors version in this bundle
	Version actors.Version
	// Release is the release id
	Release string
	// Path is the (optional) bundle path; takes precedence over url
	Path map[string]string
	// URL is the (optional) bundle URL; takes precedence over github release
	URL map[string]BundleURL
	// Devlopment indicates whether this is a development version; when set, in conjunction with path,
	// it will always load the bundle to the blockstore, without recording the manifest CID in the
	// datastore.
	Development bool
}

type BundleURL struct {
	// URL is the url of the bundle
	URL string
	// Checksum is the sha256 checksum of the bundle
	Checksum string
}

var BuiltinActorReleases map[actors.Version]Bundle

func init() {
	BuiltinActorReleases = make(map[actors.Version]Bundle)

	spec := BundleSpec{}

	r := bytes.NewReader(BuiltinActorBundles)
	_, err := toml.DecodeReader(r, &spec)
	if err != nil {
		panic(err)
	}

	for _, b := range spec.Bundles {
		BuiltinActorReleases[b.Version] = b
	}
}
