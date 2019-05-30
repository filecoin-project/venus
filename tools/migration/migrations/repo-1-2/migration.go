package migration12

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

// MetadataFormatJSONtoCBOR is the migration from version 1 to 2.
type MetadataFormatJSONtoCBOR struct {
	headKey    datastore.Key
	chainStore *chain.DefaultStore
}

// Describe describes the steps this migration will take.
func (m *MetadataFormatJSONtoCBOR) Describe() string {
	return `MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2.

    This migration changes the chain store metadata serialization from JSON to CBOR.
    The chain store metadata will be read in as JSON and rewritten as CBOR. 
	Chain store metadata consists of CIDs and the chain State Root. 
	No other repo data is changed.  Migrations are performed on a copy of the
	chain store.
`
}

// Migrate performs the migration steps
func (m *MetadataFormatJSONtoCBOR) Migrate(newRepoPath string) error {
	// open the repo path
	oldVer, _ := m.Versions()

	// This call performs some checks on the repo before we start.
	fsrepo, err := repo.OpenFSRepo(newRepoPath, oldVer)
	if err != nil {
		return err
	}

	// we assume the genesis cid on disk is valid and perform no checks or accesses
	fakeGenCid := cid.Cid{}

	// construct the chainstore from FSRepo
	m.chainStore = chain.NewDefaultStore(fsrepo.ChainDatastore(), fakeGenCid)

	// duplicate head key here to protect against future changes
	newKey := datastore.NewKey("/chain/heaviestTipSet")
	m.headKey = newKey

	if err = m.convertJSONtoCBOR(context.Background()); err != nil {
		return err
	}
	return nil
}

// Versions returns the old and new versions that are valid for this migration
func (m *MetadataFormatJSONtoCBOR) Versions() (from, to uint) {
	return 1, 2
}

// Validate performs validation tests for the migration steps
func (m *MetadataFormatJSONtoCBOR) Validate(oldRepoPath, newRepoPath string) error {
	// validate calls some kind of chain reader function that operates on the new version
	// of the chain head.
	return nil
}

// loadHeadFromStoreAsJSON loads the latest known head from disk.
func (m *MetadataFormatJSONtoCBOR) loadHeadFromStoreAsJSON() (types.SortedCidSet, error) {
	var emptyCidSet types.SortedCidSet

	bb, err := m.chainStore.GetDatastore().Get(m.headKey)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to read headKey")
	}

	var cids types.SortedCidSet
	err = json.Unmarshal(bb, &cids)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to cast headCids")
	}

	return cids, nil
}

func (m *MetadataFormatJSONtoCBOR) loadStateRootAsJSON(ts types.TipSet) (cid.Cid, error) {
	h, err := ts.Height()
	if err != nil {
		return cid.Undef, err
	}
	key := datastore.NewKey(makeKey(ts.String(), h))
	bb, err := m.chainStore.GetDatastore().Get(key)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed to read tipset key %s", ts.String())
	}

	var stateRoot cid.Cid
	err = json.Unmarshal(bb, &stateRoot)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed to cast state root of tipset %s", ts.String())
	}
	return stateRoot, nil
}

// this is pared-down from chain DefaultStore.Load (elaborate)
func (m *MetadataFormatJSONtoCBOR) convertJSONtoCBOR(ctx context.Context) error {
	tipCids, err := m.loadHeadFromStoreAsJSON()
	if err != nil {
		return err
	}
	headTs := types.TipSet{}
	// traverse starting from head to begin loading the chain
	for it := tipCids.Iter(); !it.Complete(); it.Next() {
		blk, err := m.chainStore.GetBlock(ctx, it.Value())
		if err != nil {
			return errors.Wrap(err, "failed to load block in head TipSet")
		}
		err = headTs.AddBlock(blk)
		if err != nil {
			return errors.Wrap(err, "failed to add validated block to TipSet")
		}
	}

	for iterator := chain.IterAncestors(ctx, m.chainStore, headTs); !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}
		stateRoot, err := m.loadStateRootAsJSON(iterator.Value())
		if err != nil {
			return err
		}

		tipSetAndState := &chain.TipSetAndState{
			TipSet:          iterator.Value(),
			TipSetStateRoot: stateRoot,
		}

		// only write TipSet and State; Block and tipIndex formats are not changed.
		if err = m.writeTipSetAndStateAsCBOR(tipSetAndState); err != nil {
			return err
		}
	}

	// write head.
	return m.writeHeadAsCBOR(ctx, headTs.ToSortedCidSet())
}

// writeHeadAsCBOR writes the head (taken from DefaultStore.writeHead)
func (m *MetadataFormatJSONtoCBOR) writeHeadAsCBOR(ctx context.Context, cids types.SortedCidSet) error {
	val, err := cbor.DumpObject(cids)
	if err != nil {
		return err
	}

	// this writes the value to the FSRepo
	return m.chainStore.GetDatastore().Put(m.headKey, val)
}

// writeTipSetAndStateAsCBOR writes the tipset key and the state root id to the
// datastore. (taken from DefaultStore.writeTipSetAndState)
func (m *MetadataFormatJSONtoCBOR) writeTipSetAndStateAsCBOR(tsas *chain.TipSetAndState) error {
	if tsas.TipSetStateRoot == cid.Undef {
		return errors.New("attempting to write state root cid.Undef")
	}
	val, err := cbor.DumpObject(tsas.TipSetStateRoot)
	if err != nil {
		return err
	}

	// datastore keeps tsKey:stateRoot (k,v) pairs.
	h, err := tsas.TipSet.Height()
	if err != nil {
		return err
	}
	keyStr := makeKey(tsas.TipSet.String(), h)
	key := datastore.NewKey(keyStr)

	// this writes the value to the FSRepo
	return m.chainStore.GetDatastore().Put(key, val)
}

func makeKey(pKey string, h uint64) string {
	return fmt.Sprintf("p-%s h-%d", pKey, h)
}
