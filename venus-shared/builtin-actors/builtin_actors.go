package builtinactors

import (
	"context"
	"embed"
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

type BuiltinActorsBuilder struct {
	actorsv7FS embed.FS
	actorsv8FS embed.FS

	actorsv7 []byte
	actorsv8 []byte
}

func NewBuiltinActorsBuilder(v7, v8 embed.FS) *BuiltinActorsBuilder {
	return &BuiltinActorsBuilder{
		actorsv7FS: v7,
		actorsv8FS: v8,
	}
}

func (b *BuiltinActorsBuilder) BuiltinActorsV8Bundle() []byte {
	return b.actorsv8
}

func (b *BuiltinActorsBuilder) BuiltinActorsV7Bundle() []byte {
	return b.actorsv7
}

func (b *BuiltinActorsBuilder) LoadActorsFromCar(networkType types.NetworkType) error {
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

	b.actorsv7, err = b.actorsv7FS.ReadFile(filepath.Join("v7", file))
	if err != nil {
		return xerrors.Errorf("failed load actor v7 car file %v", err)
	}

	b.actorsv8, err = b.actorsv8FS.ReadFile(filepath.Join("v8", file))
	if err != nil {
		return xerrors.Errorf("failed load actor v8 car file %v", err)
	}

	log.Debugf("actor v7 car file length %d, actor v8 car file length %d", len(b.actorsv7), len(b.actorsv8))

	return nil
}

func (b *BuiltinActorsBuilder) LoadBuiltinActors(ctx context.Context, bs blockstoreutil.Blockstore) (result BuiltinActorsLoaded, err error) {
	// TODO eventually we want this to start with bundle/manifest CIDs and fetch them from IPFS if
	//      not already loaded.
	//      For now, we just embed the v8 bundle and adjust the manifest CIDs for the migration/actor
	//      metadata.
	if len(b.BuiltinActorsV8Bundle()) > 0 {
		if err := actors.LoadBundle(ctx, bs, actors.Version8, b.BuiltinActorsV8Bundle()); err != nil {
			return result, err
		}
	}

	// for testing -- need to also set LOTUS_USE_FVM_CUSTOM_BUNDLE=1 to force the fvm to use it.
	if len(b.BuiltinActorsV7Bundle()) > 0 {
		if err := actors.LoadBundle(ctx, bs, actors.Version7, b.BuiltinActorsV7Bundle()); err != nil {
			return result, err
		}
	}

	cborStore := cbor.NewCborStore(bs)
	if err := actors.LoadManifests(ctx, cborStore); err != nil {
		return result, xerrors.Errorf("error loading actor manifests: %w", err)
	}

	return result, nil
}
