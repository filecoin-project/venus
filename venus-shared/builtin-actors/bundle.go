package builtinactors

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var NetworkBundle string
var ActorsCIDs = make(map[actors.Version]cid.Cid)

func SetBundleInfo(networkType types.NetworkType, repoPath string) error {
	if err := os.Setenv(BundleRepoPath, repoPath); err != nil {
		return err
	}
	switch networkType {
	case types.Network2k, types.NetworkForce:
		NetworkBundle = "devnet"
	case types.NetworkButterfly:
		NetworkBundle = "butterflynet"
		ActorsCIDs[actors.Version8] = types.MustParseCid("bafy2bzacedy4qgxbr6pbyfgcp7s7bdkc2whi5errnw67al5e2tk75j46iucv6")
	case types.NetworkInterop:
		NetworkBundle = "caterpillarnet"
		ActorsCIDs[actors.Version8] = types.MustParseCid("bafy2bzacebjfypksxfieex7d6rh27lu2dk2m7chnok5mtzigj3txvb7leqbys")
	case types.NetworkCalibnet:
		NetworkBundle = "calibrationnet"
	default:
		NetworkBundle = "mainnet"
	}
	fmt.Printf("NetworkBundle %v, BUNDLE_REPO_PATH: %v, ActorsCIDs: %v \n", NetworkBundle, repoPath, ActorsCIDs)
	return nil
}

// Must have called SetBundleInfo before calling GetActorsCIDs
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
