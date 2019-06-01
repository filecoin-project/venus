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

// duplicate head key here to protect against future changes
var headKey = datastore.NewKey("/chain/heaviestTipSet")

// migrationDefaultStore is a stripped down implementation of the Store interface
// based on DefaultStore, containing only the fields needed for the migration.
//
// the extraction line is drawn at package level where the migration occurs, i.e. chain,
// and no further.
type migrationDefaultStore struct {

	// bsPriv is the on disk storage for blocks.  This is private to
	// the DefaultStore to keep code that adds blocks to the DefaultStore's
	// underlying storage isolated to this module.  It is important that only
	// code with access to a DefaultStore can write to this storage to
	// simplify checking the security guarantee that only tipsets of a
	// validated chain are stored in the filecoin node's DefaultStore.
	BsPriv bstore.Blockstore

	// ds is the datastore backing bsPriv.  It is also accessed directly
	// to set and get chain meta-data, specifically the tipset cidset to
	// state root mapping, and the heaviest tipset cids.
	Ds repo.Datastore

	// head is the tipset at the head of the best known chain.
	Head types.TipSet

	// Tracks tipsets by height/parentset for use by expected consensus.
	TipIndex *chain.TipIndex
}

// GetBlock retrieves a block by cid.
func (store *migrationDefaultStore) GetBlock(ctx context.Context, c cid.Cid) (*types.Block, error) {
	data, err := store.BsPriv.Get(c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %s", c.String())
	}
	return types.DecodeBlock(data.RawData())
}

// MetadataFormatJSONtoCBOR is the migration from version 1 to 2.
type MetadataFormatJSONtoCBOR struct {
	chainStore *migrationDefaultStore
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
	defer mustCloseRepo(fsrepo)

	// construct the chainstore from FSRepo
	m.chainStore = &migrationDefaultStore{
		BsPriv:   bstore.NewBlockstore(fsrepo.ChainDatastore()),
		Ds:       fsrepo.ChainDatastore(),
		TipIndex: chain.NewTipIndex(),
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
// compares old/new chain heads and state roots,
// returns error if either is not equal.
func (m *MetadataFormatJSONtoCBOR) Validate(oldRepoPath, newRepoPath string) error {
	// open the repo path
	oldVer, newVer := m.Versions()

	// This call performs some checks on the repo before we start.
	oldFsRepo, err := repo.OpenFSRepo(oldRepoPath, oldVer)
	if err != nil {
		return err
	}
	defer mustCloseRepo(oldFsRepo)

	// construct the chainstore from FSRepo
	oldChainStore := &migrationDefaultStore{
		BsPriv:   bstore.NewBlockstore(oldFsRepo.ChainDatastore()),
		Ds:       oldFsRepo.ChainDatastore(),
		TipIndex: chain.NewTipIndex(),
	}

	newFsRepo, err := repo.OpenFSRepo(newRepoPath, newVer)
	if err != nil {
		return err
	}
	defer mustCloseRepo(newFsRepo)

	newChainStore := &migrationDefaultStore{
		BsPriv:   bstore.NewBlockstore(newFsRepo.ChainDatastore()),
		Ds:       newFsRepo.ChainDatastore(),
		TipIndex: chain.NewTipIndex(),
	}

	// TODO: won't need to compare these until the end and won't need to call
	// loadChainHead explicitly once the other stuff is loaded
	// get tipset and state from old (JSON)
	oldChainHead, err := loadChainHead(false, oldChainStore)
	if err != nil {
		return err
	}

	newChainHead, err := loadChainHead(true, newChainStore)
	if err != nil {
		return err
	}

	// TODO: call the loadChainStore fcn here
	err = loadChainStore(context.Background(), false, oldChainStore)
	if err != nil {
		return err
	}

	if !oldChainHead.Equals(newChainHead) {
		return errors.New("migrated chain head not equal to source chain head")
	}

	// get tipset and state from new (CBOR)
	// error if head or stateRoots are not equal
	return nil
}

// loadChainHead loads the chain head as either CBOR or JSON
func loadChainHead(asCBOR bool, chainStore *migrationDefaultStore) (types.SortedCidSet, error) {
	var emptyCidSet types.SortedCidSet

	bb, err := chainStore.Ds.Get(headKey)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to read headKey")
	}

	var cids types.SortedCidSet
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

// loadChainHeadAsCBOR loads the latest known head from disk assuming CBOR format
func (m *MetadataFormatJSONtoCBOR) loadChainHeadAsCBOR() (types.SortedCidSet, error) {
	return loadChainHead(true, m.chainStore)
}

// loadChainHeadAsJSON loads the latest known head from disk assuming JSON format
func (m *MetadataFormatJSONtoCBOR) loadChainHeadAsJSON() (types.SortedCidSet, error) {
	return loadChainHead(false, m.chainStore)
}

func loadStateRoot(ts types.TipSet, asCBOR bool, chainStore *migrationDefaultStore) (cid.Cid, error) {
	h, err := ts.Height()
	if err != nil {
		return cid.Undef, err
	}
	key := datastore.NewKey(makeKey(ts.String(), h))
	bb, err := chainStore.Ds.Get(key)
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

func (m *MetadataFormatJSONtoCBOR) loadStateRootAsJSON(ts types.TipSet) (cid.Cid, error) {
	return loadStateRoot(ts, false, m.chainStore)
}

func (m *MetadataFormatJSONtoCBOR) loadStateRootAsCBOR(ts types.TipSet) (cid.Cid, error) {
	return loadStateRoot(ts, true, m.chainStore)
}

func loadChainStore(ctx context.Context, asCBOR bool, chainStore *migrationDefaultStore) error {
	tipCids, err := loadChainHead(asCBOR, chainStore)
	if err != nil {
		return err
	}
	headTs := types.TipSet{}
	// traverse starting from head to begin loading the chain
	for it := tipCids.Iter(); !it.Complete(); it.Next() {
		blk, err := chainStore.GetBlock(ctx, it.Value())
		if err != nil {
			return errors.Wrap(err, "failed to load block in head TipSet")
		}
		err = headTs.AddBlock(blk)
		if err != nil {
			return errors.Wrap(err, "failed to add validated block to TipSet")
		}
	}

	for iterator := chain.IterAncestors(ctx, chainStore, headTs); !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return err
		}

		stateRoot, err := loadStateRoot(iterator.Value(), asCBOR, chainStore)
		if err != nil {
			return err
		}

		tipSetAndState := &chain.TipSetAndState{
			TipSet:          iterator.Value(),
			TipSetStateRoot: stateRoot,
		}

		err = chainStore.TipIndex.Put(tipSetAndState)
		if err != nil {
			return err
		}
	}

	// set chain head
	chainStore.Head = headTs
	return nil
}

// convertJSONtoCBOR is adapted from chain DefaultStore.Load:
//     1. stripped out logging
//     2. instead of calling DefaultStore.PutTipSetAndState it just calls
//        writeTipSetAndStateAsCBOR, because block format is unchanged.
//     3. then calls writeHeadAsCBOR, instead of store.SetHead which also publishes an event
//        and does some logging
//
// This migration will leave some fork metadata in JSON format in the repo, BUT it won't matter:
//   for consensus purposes we don't care about uncle blocks, and if we see the block again,
//   it will be over the network, then DataStore.Put will look for it as CBOR, won't find it and write it out as CBOR anyway.  If it's never seen again we don't care about it.
func (m *MetadataFormatJSONtoCBOR) convertJSONtoCBOR(ctx context.Context) error {
	tipCids, err := m.loadChainHeadAsJSON()
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

		stateRoot, err := m.loadStateRootAsJSON(iterator.Value())
		if err != nil {
			return err
		}

		tipSetAndState := &chain.TipSetAndState{
			TipSet:          iterator.Value(),
			TipSetStateRoot: stateRoot,
		}

		// Maybe not necessary but snippet could be useful for Validate
		//err = m.chainStore.TipIndex.Put(tipSetAndState)
		//if err != nil {
		//	return err
		//}

		// only write TipSet and State; Block and tipIndex formats are not changed.
		if err = m.writeTipSetAndStateAsCBOR(tipSetAndState); err != nil {
			return err
		}

	}

	// write head.
	err = m.writeHeadAsCBOR(ctx, headTs.ToSortedCidSet())
	if err != nil {
		return err
	}

	// set chain head
	// Maybe not necessary but snippet could be useful for validate
	//m.chainStore.Head = headTs
	return nil
}

// writeHeadAsCBOR writes the head (taken from DefaultStore.writeHead, which was called by
// setHeadPersistent. We don't need mutexes for this
func (m *MetadataFormatJSONtoCBOR) writeHeadAsCBOR(ctx context.Context, cids types.SortedCidSet) error {
	val, err := cbor.DumpObject(cids)
	if err != nil {
		return err
	}

	// this writes the value to the FSRepo
	return m.chainStore.Ds.Put(headKey, val)
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
	return m.chainStore.Ds.Put(key, val)
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
