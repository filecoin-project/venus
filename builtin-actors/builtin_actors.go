package builtinactors

import (
	"context"
	"embed"
	"path/filepath"

	"github.com/filecoin-project/venus/pkg/constants"

	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

var log = logging.Logger("builtin-actors")

//go:embed v8
var actorsv8FS embed.FS

var actorsv8 []byte

func BuiltinActorsV8Bundle() []byte {
	return actorsv8
}

//go:embed v7
var actorsv7FS embed.FS

var actorsv7 []byte

func BuiltinActorsV7Bundle() []byte {
	return actorsv7
}

func LoadActorsFromCar(networkType constants.NetworkType) error {
	file := ""
	switch networkType {
	case constants.Network2k, constants.NetworkForce:
		file = "builtin-actors-devnet.car"
	case constants.NetworkButterfly:
		file = "builtin-actors-butterflynet.car"
	case constants.NetworkInterop:
		file = "builtin-actors-caterpillarnet.car"
	case constants.NetworkCalibnet:
		file = "builtin-actors-calibrationnet.car"
	case constants.NetworkMainnet:
		file = "builtin-actors-mainnet.car"
	}
	if len(file) == 0 {
		return xerrors.Errorf("unexpected network type %d", networkType)
	}
	var err error

	actorsv7, err = actorsv7FS.ReadFile(filepath.Join("v7", file))
	if err != nil {
		return xerrors.Errorf("failed load actor v7 car file %v", err)
	}

	actorsv8, err = actorsv8FS.ReadFile(filepath.Join("v8", file))
	if err != nil {
		return xerrors.Errorf("failed load actor v8 car file %v", err)
	}

	log.Debugf("actor v7 car file length %d, actor v8 car file length %d", len(actorsv7), len(actorsv8))

	return nil
}

type BuiltinActorsLoaded struct{}

func LoadBuiltinActors(ctx context.Context) (result BuiltinActorsLoaded, err error) {
	bs := blockstoreutil.NewMemory()
	// TODO eventually we want this to start with bundle/manifest CIDs and fetch them from IPFS if
	//      not already loaded.
	//      For now, we just embed the v8 bundle and adjust the manifest CIDs for the migration/actor
	//      metadata.
	if len(BuiltinActorsV8Bundle()) > 0 {
		if err := actors.LoadBundle(ctx, bs, actors.Version8, BuiltinActorsV8Bundle()); err != nil {
			return result, err
		}
	}

	// for testing -- need to also set LOTUS_USE_FVM_CUSTOM_BUNDLE=1 to force the fvm to use it.
	if len(BuiltinActorsV7Bundle()) > 0 {
		if err := actors.LoadBundle(ctx, bs, actors.Version7, BuiltinActorsV7Bundle()); err != nil {
			return result, err
		}
	}

	cborStore := cbor.NewCborStore(bs)
	if err := actors.LoadManifests(ctx, cborStore); err != nil {
		return result, xerrors.Errorf("error loading actor manifests: %w", err)
	}

	return result, nil
}
