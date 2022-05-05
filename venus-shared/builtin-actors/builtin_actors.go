package builtinactors

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var log = logging.Logger("builtin-actors")

type BuiltinActorsLoaded struct{}

var (
	actorsv7 []byte
	actorsv8 []byte
)

func BuiltinActorsV8Bundle() []byte {
	return actorsv8
}

func BuiltinActorsV7Bundle() []byte {
	return actorsv7
}

func SetActorsBundle(actorsv7FS, actorsv8FS embed.FS, networkType types.NetworkType) error {
	file := ""
	switch networkType {
	case types.Network2k, types.NetworkForce:
		file = "builtin-actors-devnet.car"
	case types.NetworkButterfly:
		file = "builtin-actors-butterflynet.car"
	case types.NetworkInterop:
		file = "builtin-actors-caterpillarnet.car"
	case types.NetworkCalibnet:
		file = "builtin-actors-calibrationnet.car"
	case types.NetworkMainnet:
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

func LoadBuiltinActors(ctx context.Context, bs blockstoreutil.Blockstore) (result BuiltinActorsLoaded, err error) {
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

// for test
func LoadBuiltinActorsTesting(ctx context.Context, bs blockstoreutil.Blockstore, insecurePoStValidation bool) (BuiltinActorsLoaded, error) {
	base := os.Getenv("VENUS_SRC_DIR")
	if base == "" {
		base = "."
	}

	var template string
	if insecurePoStValidation {
		template = "%s/builtin-actors/v%d/builtin-actors-testing-fake-proofs.car"
	} else {
		template = "%s/builtin-actors/v%d/builtin-actors-testing.car"
	}

	for _, ver := range []actors.Version{actors.Version8} {
		path := fmt.Sprintf(template, base, ver)

		log.Infof("loading testing bundle: %s", path)

		file, err := os.Open(path)
		if err != nil {
			return BuiltinActorsLoaded{}, xerrors.Errorf("error opening v%d bundle: %w", ver, err)
		}

		bundle, err := io.ReadAll(file)
		if err != nil {
			return BuiltinActorsLoaded{}, xerrors.Errorf("error reading v%d bundle: %w", ver, err)
		}

		if err := actors.LoadBundle(ctx, bs, actors.Version8, bundle); err != nil {
			return BuiltinActorsLoaded{}, xerrors.Errorf("error loading v%d bundle: %w", ver, err)
		}
	}

	cborStore := cbor.NewCborStore(bs)
	if err := actors.LoadManifests(ctx, cborStore); err != nil {
		return BuiltinActorsLoaded{}, xerrors.Errorf("error loading actor manifests: %w", err)
	}

	return BuiltinActorsLoaded{}, nil
}
