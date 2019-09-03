package migration12

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

// This migration was written for a prior development state in order to demonstrate migration
// functionality and workflow. It's not expected to run against a current repo and the tests have
// since been removed. This code remains for the next migration author to use as a reference,
// after which we can probably remove it.

// duplicate head key here to protect against future changes
var headKey = datastore.NewKey("/chain/heaviestTipSet")

// migrationChainStore is a stripped down implementation of the Store interface
// based on Store, containing only the fields and functions needed for the migration.
//
// the extraction line is drawn at package level where the migration occurs, i.e. chain,
// and no further.
type migrationChainStore struct {
	// BsPriv is the on disk storage for blocks.
	BsPriv bstore.Blockstore

	// Ds is the datastore backing bsPriv.  It is also accessed directly
	// to set and get chain meta-data, specifically the tipset cidset to
	// state root mapping, and the heaviest tipset cids.
	Ds repo.Datastore

	// head is the tipset at the head of the best known chain.
	Head types.TipSet
}

// GetBlock retrieves a block by cid.
func (store *migrationChainStore) GetBlock(ctx context.Context, c cid.Cid) (*types.Block, error) {
	data, err := store.BsPriv.Get(c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %s", c.String())
	}
	return types.DecodeBlock(data.RawData())
}

// MetadataFormatJSONtoCBOR is the migration from version 1 to 2.
type MetadataFormatJSONtoCBOR struct {
	store *migrationChainStore
}

// Describe describes the steps this migration will take.
func (m *MetadataFormatJSONtoCBOR) Describe() string {
	return `MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2.

    This migration changes chain store metadata serialization from JSON to CBOR.
    The chain store metadata will be read in as JSON and rewritten as CBOR. 
	Chain store metadata consists of associations between tipset keys and state 
	root cids and the tipset key of the head of the chain. No other repo data is changed.
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
	defer mustCloseRepo(fsrepo)

	// construct the chainstore to be migrated from FSRepo
	m.store = &migrationChainStore{
		BsPriv: bstore.NewBlockstore(fsrepo.ChainDatastore()),
		Ds:     fsrepo.ChainDatastore(),
	}

	if err = m.convertJSONtoCBOR(context.Background()); err != nil {
		return err
	}
	return nil
}

// Versions returns the old and new versions that are valid for this migration
func (m *MetadataFormatJSONtoCBOR) Versions() (from, to uint) {
	return 1, 2
}

// Validate performs validation tests for the migration steps:
// Reads in the old chainstore and the new chainstore,
// Compares the two and returns error if they are not completely equal once loaded.
func (m *MetadataFormatJSONtoCBOR) Validate(oldRepoPath, newRepoPath string) error {
	// open the repo path
	oldVer, _ := m.Versions()

	// This call performs some checks on the repo before we start.
	oldFsRepo, err := repo.OpenFSRepo(oldRepoPath, oldVer)
	if err != nil {
		return err
	}
	defer mustCloseRepo(oldFsRepo)

	// construct the chainstore from FSRepo
	oldStore := &migrationChainStore{
		BsPriv: bstore.NewBlockstore(oldFsRepo.ChainDatastore()),
		Ds:     oldFsRepo.ChainDatastore(),
	}

	// Version hasn't been updated yet.
	newFsRepo, err := repo.OpenFSRepo(newRepoPath, oldVer)
	if err != nil {
		return err
	}
	defer mustCloseRepo(newFsRepo)

	newStore := &migrationChainStore{
		BsPriv: bstore.NewBlockstore(newFsRepo.ChainDatastore()),
		Ds:     newFsRepo.ChainDatastore(),
	}

	ctx := context.Background()

	// compare entire chainstores
	if err = compareChainStores(ctx, oldStore, newStore); err != nil {
		return errors.Wrap(err, "old and new chainStores are not equal")
	}

	return nil
}

// convertJSONtoCBOR is adapted from chain Store.Load:
//     1. stripped out logging
//     2. instead of calling Store.PutTipSetAndState it just calls
//        writeTipSetAndStateAsCBOR, because block format is unchanged.
//     3. then calls writeHeadAsCBOR, instead of store.SetHead which also publishes an event
//        and does some logging
//
// This migration will leave some fork metadata in JSON format in the repo, but it won't matter:
//   for consensus purposes we don't care about uncle blocks, and if we see the block again,
//   it will be over the network, then DataStore.Put will look for it as CBOR, won't find it and write it out as CBOR anyway.  If it's never seen again we don't care about it.
func (m *MetadataFormatJSONtoCBOR) convertJSONtoCBOR(ctx context.Context) error {
	tipCids, err := loadChainHeadAsJSON(m.store)
	if err != nil {
		return err
	}
	var blocks []*types.Block
	// traverse starting from head to begin loading the chain
	for it := tipCids.Iter(); !it.Complete(); it.Next() {
		blk, err := m.store.GetBlock(ctx, it.Value())
		if err != nil {
			return errors.Wrap(err, "failed to load block in head TipSet")
		}
		blocks = append(blocks, blk)
	}

	headTs, err := types.NewTipSet(blocks...)
	if err != nil {
		return errors.Wrap(err, "failed to add validated block to TipSet")
	}

	tipsetProvider := chain.TipSetProviderFromBlocks(ctx, m.store)
	for iter := chain.IterAncestors(ctx, tipsetProvider, headTs); !iter.Complete(); err = iter.Next() {
		if err != nil {
			return err
		}

		stateRoot, err := loadStateRootAsJSON(iter.Value(), m.store)
		if err != nil {
			return err
		}
		tipSetAndState := &chain.TipSetAndState{
			TipSet:          iter.Value(),
			TipSetStateRoot: stateRoot,
		}
		// only write TipSet and State; Block and tipIndex formats are not changed.
		if err = m.writeTipSetAndStateAsCBOR(tipSetAndState); err != nil {
			return err
		}
	}

	// write head back out as CBOR
	err = m.writeHeadAsCBOR(ctx, headTs.Key())
	if err != nil {
		return err
	}
	return nil
}

// writeHeadAsCBOR writes the head. Taken from Store.writeHead, which was called by
// setHeadPersistent. We don't need mutexes for this
func (m *MetadataFormatJSONtoCBOR) writeHeadAsCBOR(ctx context.Context, cids types.TipSetKey) error {
	val, err := cbor.DumpObject(cids)
	if err != nil {
		return err
	}

	// this writes the value to the FSRepo
	return m.store.Ds.Put(headKey, val)
}

// writeTipSetAndStateAsCBOR writes the tipset key and the state root id to the
// datastore. (taken from Store.writeTipSetAndState)
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
	return m.store.Ds.Put(key, val)
}

// loadChainHeadAsJSON loads the latest known head from disk assuming JSON format
func loadChainHeadAsJSON(chainStore *migrationChainStore) (types.TipSetKey, error) {
	return loadChainHead(false, chainStore)
}

// loadChainHeadAsCBOR loads the latest known head from disk assuming CBOR format
func loadChainHeadAsCBOR(store *migrationChainStore) (types.TipSetKey, error) {
	return loadChainHead(true, store)
}

// loadChainHead loads the chain head CIDs as either CBOR or JSON
func loadChainHead(asCBOR bool, store *migrationChainStore) (types.TipSetKey, error) {
	var emptyCidSet types.TipSetKey

	bb, err := store.Ds.Get(headKey)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to read headKey")
	}

	var cids types.TipSetKey
	if asCBOR {
		err = cbor.DecodeInto(bb, &cids)

	} else {
		err = json.Unmarshal(bb, &cids)
	}
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to cast headCids")
	}

	return cids, nil
}

// function wrapper for loading state root as JSON
func loadStateRootAsJSON(ts types.TipSet, store *migrationChainStore) (cid.Cid, error) {
	return loadStateRoot(ts, false, store)
}

// loadStateRoot loads the chain store metadata into store, updating its
// state root and then returning the state root + any error
// pass true to load as CBOR (new format) or false to load as JSON (old format)
func loadStateRoot(ts types.TipSet, asCBOR bool, store *migrationChainStore) (cid.Cid, error) {
	h, err := ts.Height()
	if err != nil {
		return cid.Undef, err
	}
	key := datastore.NewKey(makeKey(ts.String(), h))
	bb, err := store.Ds.Get(key)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed to read tipset key %s", ts.String())
	}

	var stateRoot cid.Cid
	if asCBOR {
		err = cbor.DecodeInto(bb, &stateRoot)
	} else {
		err = json.Unmarshal(bb, &stateRoot)
	}
	if err != nil {
		return cid.Undef, errors.Wrapf(err, "failed to cast state root of tipset %s", ts.String())
	}
	return stateRoot, nil
}

// compareChainStores loads each chain store and iterates through each, comparing heads
// then state root at each iteration, returning error if:
//     * at any point state roots are not equal
//     * the old and new chain store iterators are not complete at the same time
// 	   * at the end the two heads are not equal
func compareChainStores(ctx context.Context, oldStore *migrationChainStore, newStore *migrationChainStore) error {
	oldTipCids, err := loadChainHeadAsJSON(oldStore)
	if err != nil {
		return err
	}

	newTipCids, err := loadChainHeadAsCBOR(newStore)
	if err != nil {
		return err
	}

	oldHeadTs, err := loadTipSet(ctx, oldTipCids, oldStore)
	if err != nil {
		return err
	}

	newHeadTs, err := loadTipSet(ctx, newTipCids, newStore)
	if err != nil {
		return err
	}

	if !newHeadTs.Equals(oldHeadTs) {
		return errors.New("new and old head tipsets not equal")
	}

	oldIt := chain.IterAncestors(ctx, chain.TipSetProviderFromBlocks(ctx, oldStore), oldHeadTs)
	for newIt := chain.IterAncestors(ctx, chain.TipSetProviderFromBlocks(ctx, newStore), newHeadTs); !newIt.Complete(); err = newIt.Next() {
		if err != nil {
			return err
		}
		if oldIt.Complete() {
			return errors.New("old chain store is shorter than new chain store")
		}

		newSr, err := loadStateRoot(newIt.Value(), true, newStore)
		if err != nil {
			return err
		}

		oldSr, err := loadStateRootAsJSON(oldIt.Value(), oldStore)
		if err != nil {
			return err
		}
		if !newSr.Equals(oldSr) {
			return errors.New("current state root not equal for block")
		}
		if err = oldIt.Next(); err != nil {
			return errors.Wrap(err, "old chain store Next failed")
		}
	}
	if !oldIt.Complete() {
		return errors.New("old chain store is longer than new chain store")
	}
	return nil
}

func loadTipSet(ctx context.Context, cidSet types.TipSetKey, chainStore *migrationChainStore) (headTs types.TipSet, err error) {
	var blocks []*types.Block
	for iter := cidSet.Iter(); !iter.Complete(); iter.Next() {
		blk, err := chainStore.GetBlock(ctx, iter.Value())
		if err != nil {
			return headTs, errors.Wrap(err, "failed to load block in head TipSet")
		}
		blocks = append(blocks, blk)
	}
	headTs, err = types.NewTipSet(blocks...)
	if err != nil {
		return headTs, err
	}
	return headTs, nil
}

func makeKey(pKey string, h uint64) string {
	return fmt.Sprintf("p-%s h-%d", pKey, h)
}

func mustCloseRepo(fsRepo *repo.FSRepo) {
	err := fsRepo.Close()
	if err != nil {
		panic(err)
	}
}
