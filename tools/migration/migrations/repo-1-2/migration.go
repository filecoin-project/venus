package migration12

import (
	"context"
	"encoding/json"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbornode "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

var headKey = datastore.NewKey("/chain/heaviestTipSet")

// MetadataFormatJSONtoCBOR is the migration from version 1 to 2.
type MetadataFormatJSONtoCBOR struct {
	chainStore *chain.DefaultStore
}

// Describe describes the steps this migration will take.
func (m *MetadataFormatJSONtoCBOR) Describe() string {
	return `MetadataFormatJSONtoCBOR migrates the storage repo from version 1 to 2.

    This migration changes the chain store metadata serialization from JSON to CBOR.
    Your chain store metadata will be read in as JSON and rewritten as CBOR. 
	Chain store metadata consists of CIDs and the chain State Root. 
	No other repo data is changed.  Migrations are performed on a copy of your
	chain store.
`
}

// Migrate performs the migration steps
func (m *MetadataFormatJSONtoCBOR) Migrate(newRepoPath string) error {
	// open the repo path
	oldVer, _ := m.Versions()

	// This performs some checks on the repo before we start
	fsrepo, err := repo.OpenFSRepo(newRepoPath, oldVer)
	if err != nil {
		return err
	}

	//genCid, err := m.readGenesisCid(fsrepo.Datastore())

	m.chainStore = chain.NewDefaultStore(fsrepo.ChainDatastore(), cid.Cid{})

	// read the metadata as JSON
	cidset, err := m.loadHeadFromStore()
	if err != nil {
		return err
	}

	// write as CBOR
	if err = m.writeHead(context.Background(), cidset); err != nil {
		return err
	}

	// done
	return nil
}

// Versions returns the old and new versions that are valid for this migration
func (m *MetadataFormatJSONtoCBOR) Versions() (from, to uint) {
	return 1, 2
}

// Validate performs validation tests for the migration steps
func (m *MetadataFormatJSONtoCBOR) Validate(oldRepoPath, newRepoPath string) error {
	return nil
}

func (m *MetadataFormatJSONtoCBOR) loadHeadFromStore() (types.SortedCidSet, error) {
	var emptyCidSet types.SortedCidSet

	bb, err := m.chainStore.GetDatastore().Get(headKey)
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

// readGenesisCid is a helper function that queries the provided datastore for
// an entry with the genesisKey cid, returning if found.
//func (m *MetadataFormatJSONtoCBOR)readGenesisCid(ds datastore.Datastore) (cid.Cid, error) {
//	bb, err := ds.Get(chain.GenesisKey)
//	if err != nil {
//		return cid.Undef, errors.Wrap(err, "failed to read genesisKey")
//	}
//
//	var c cid.Cid
//	err = json.Unmarshal(bb, &c)
//	if err != nil {
//		return cid.Undef, errors.Wrap(err, "failed to cast genesisCid")
//	}
//	return c, nil
//}

func (m *MetadataFormatJSONtoCBOR) writeHead(ctx context.Context, cids types.SortedCidSet) error {
	val, err := cbornode.DumpObject(cids)
	if err != nil {
		return err
	}

	return m.chainStore.GetDatastore.Put(headKey, val)
}
