package builtinactors

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	dstore "github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

type BuiltinActorsLoaded struct{}

func LoadBuiltinActors(ctx context.Context, repoPath string, bs blockstoreutil.Blockstore, ds dstore.Batching) (result BuiltinActorsLoaded, err error) {
	// We can't put it as a dep in inputs causes a stack overflow in DI from circular dependency
	// So we pass it through ldflags instead
	netw := NetworkBundle
	if netw == "" {
		netw = "mainnet"
	}

	for av, rel := range BuiltinActorReleases {
		// first check to see if we know this release
		key := dstore.NewKey(fmt.Sprintf("/builtin-actors/v%d/%s", av, rel))

		data, err := ds.Get(ctx, key)
		switch err {
		case nil:
			// ok, we do, this should be the manifest cid
			mfCid, err := cid.Cast(data)
			if err != nil {
				return result, xerrors.Errorf("error parsing cid for %s: %w", key, err)
			}

			// check the blockstore for existence of the manifest
			has, err := bs.Has(ctx, mfCid)
			if err != nil {
				return result, xerrors.Errorf("error checking blockstore for manifest cid %s: %w", mfCid, err)
			}

			if has {
				// it's there, no need to reload the bundle to the blockstore; just add it to the manifest list.
				actors.AddManifest(av, mfCid)
				continue
			}

			// we have a release key but don't have the manifest in the blockstore; maybe the user
			// nuked his blockstore to restart from a snapshot. So fallthrough to refetch (if necessary)
			// and reload the bundle.

		case dstore.ErrNotFound:
			// we don't have a release key, we need to load the bundle

		default:
			return result, xerrors.Errorf("error loading %s from datastore: %w", key, err)
		}

		// ok, we don't have it -- fetch it and add it to the blockstore
		mfCid, err := FetchAndLoadBundle(ctx, repoPath, bs, av, rel, netw)
		if err != nil {
			return result, err
		}

		// add the release key with the manifest to avoid reloading it in next restart.
		if err := ds.Put(ctx, key, mfCid.Bytes()); err != nil {
			return result, xerrors.Errorf("error storing manifest CID for builtin-actors vrsion %d to the datastore: %w", av, err)
		}
	}

	// we've loaded all the bundles, now load the manifests to get actor code CIDs.
	cborStore := cbor.NewCborStore(bs)
	if err := actors.LoadManifests(ctx, cborStore); err != nil {
		return result, xerrors.Errorf("error loading actor manifests: %w", err)
	}

	return result, nil
}

// for tests
var testingBundleMx sync.Mutex

func LoadBuiltinActorsTesting(ctx context.Context, bs blockstoreutil.Blockstore, insecurePoStValidation bool) (result BuiltinActorsLoaded, err error) {
	var netw string
	if insecurePoStValidation {
		netw = "testing-fake-proofs"
	} else {
		netw = "testing"
	}

	testingBundleMx.Lock()
	defer testingBundleMx.Unlock()

	for av, rel := range BuiltinActorReleases {
		const basePath = "/tmp/venus-testing"

		if _, err := FetchAndLoadBundle(ctx, basePath, bs, av, rel, netw); err != nil {
			return result, xerrors.Errorf("error loading bundle for builtin-actors vresion %d: %w", av, err)
		}
	}

	cborStore := cbor.NewCborStore(bs)
	if err := actors.LoadManifests(ctx, cborStore); err != nil {
		return result, xerrors.Errorf("error loading actor manifests: %w", err)
	}

	return result, nil
}
