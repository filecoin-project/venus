package builtinactors

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/ipfs/go-cid"
	dstore "github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

type BuiltinActorsLoaded struct{} // nolint

func LoadBuiltinActors(ctx context.Context, repoPath string, bs blockstoreutil.Blockstore, ds dstore.Batching) (result BuiltinActorsLoaded, err error) {
	// We can't put it as a dep in inputs causes a stack overflow in DI from circular dependency
	// So we pass it through ldflags instead
	netw := NetworkBundle

	for av, bd := range BuiltinActorReleases {
		// first check to see if we know this release
		key := dstore.NewKey(fmt.Sprintf("/builtin-actors/v%d/%s/%s", av, bd.Release, netw))

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

		// we haven't recorded it in the datastore, so we need to load it
		envvar := fmt.Sprintf("VENUS_BUILTIN_ACTORS_V%d_BUNDLE", av)
		var mfCid cid.Cid
		switch {
		case os.Getenv(envvar) != "":
			path := os.Getenv(envvar)

			mfCid, err = LoadBundle(ctx, bs, path, av)
			if err != nil {
				return result, err
			}

		case bd.Path[netw] != "":
			// this is a local bundle, load it directly from the filessystem
			mfCid, err = LoadBundle(ctx, bs, bd.Path[netw], av)
			if err != nil {
				return result, err
			}

		case bd.URL[netw].URL != "":
			// fetch it from the specified URL
			mfCid, err = FetchAndLoadBundleFromURL(ctx, repoPath, bs, av, bd.Release, netw, bd.URL[netw].URL, bd.URL[netw].Checksum)
			if err != nil {
				return result, err
			}

		case bd.Release != "":
			// fetch it and add it to the blockstore
			mfCid, err = FetchAndLoadBundleFromRelease(ctx, repoPath, bs, av, bd.Release, netw)
			if err != nil {
				return result, err
			}

		default:
			return result, xerrors.Errorf("no release or path specified for version %d bundle", av)
		}

		if bd.Development || bd.Release == "" {
			// don't store the release key so that we always load development bundles
			continue
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

	const basePath = "/tmp/venus-testing"
	for av, bd := range BuiltinActorReleases {
		switch {
		case bd.Path[netw] != "":
			if _, err := LoadBundle(ctx, bs, bd.Path[netw], av); err != nil {
				return result, xerrors.Errorf("error loading testing bundle for builtin-actors version %d/%s: %w", av, netw, err)
			}

		case bd.URL[netw].URL != "":
			// fetch it from the specified URL
			if _, err := FetchAndLoadBundleFromURL(ctx, basePath, bs, av, bd.Release, netw, bd.URL[netw].URL, bd.URL[netw].Checksum); err != nil {
				return result, err
			}

		case bd.Release != "":
			if _, err := FetchAndLoadBundleFromRelease(ctx, basePath, bs, av, bd.Release, netw); err != nil {
				return result, xerrors.Errorf("error loading testing bundle for builtin-actors version %d/%s: %w", av, netw, err)
			}

		default:
			return result, xerrors.Errorf("no path or release specified for version %d testing bundle", av)
		}
	}

	cborStore := cbor.NewCborStore(bs)
	if err := actors.LoadManifests(ctx, cborStore); err != nil {
		return result, xerrors.Errorf("error loading actor manifests: %w", err)
	}

	return result, nil
}
