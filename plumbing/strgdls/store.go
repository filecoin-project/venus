package strgdls

import (
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/repo"
)

// Store is plumbing implementation querying deals
type Store struct {
	dealsDs repo.Datastore
}

// StorageDealPrefix is the datastore prefix for storage deals
const StorageDealPrefix = "storagedeals"

// New returns a new Store.
func New(dealsDatastore repo.Datastore) *Store {
	return &Store{dealsDs: dealsDatastore}
}

// Iterator returns an iterator with deals matching the given query
func (store *Store) Iterator() (*query.Results, error) {
	results, err := store.dealsDs.Query(query.Query{Prefix: "/" + StorageDealPrefix})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query deals from datastore")
	}
	return &results, nil
}

// Put puts the deal into the datastore
func (store *Store) Put(storageDeal *storagedeal.Deal) error {
	proposalCid := storageDeal.Response.ProposalCid
	datum, err := cbor.DumpObject(storageDeal)
	if err != nil {
		return errors.Wrap(err, "could not marshal storageDeal")
	}

	key := datastore.KeyWithNamespaces([]string{StorageDealPrefix, proposalCid.String()})
	err = store.dealsDs.Put(key, datum)
	if err != nil {
		return errors.Wrap(err, "could not save storage deal to disk")
	}

	return nil
}
