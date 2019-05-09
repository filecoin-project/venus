package strgdls

import (
	"context"

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

// StorageDealLsResult represents a result from the storage deal Ls method. This
// can either be an error or a storage deal.
type StorageDealLsResult struct {
	Deal storagedeal.Deal
	Err  error
}

// Ls returns a channel with deals matching the given query, with a possible error
func (store *Store) Ls(ctx context.Context) (<-chan *StorageDealLsResult, error) {
	out := make(chan *StorageDealLsResult)

	results, err := store.dealsDs.Query(query.Query{Prefix: "/" + StorageDealPrefix})
	if err != nil {
		return nil, errors.Wrap(err, "failed to query deals from datastore")
	}
	go func() {
		for entry := range results.Next() {
			select {
			case <-ctx.Done():
				out <- &StorageDealLsResult{
					Err: ctx.Err(),
				}
				return
			default:
				var storageDeal storagedeal.Deal
				if err := cbor.DecodeInto(entry.Value, &storageDeal); err != nil {
					out <- &StorageDealLsResult{
						Err: errors.Wrap(err, "failed to unmarshal deals from datastore"),
					}
					return
				}
				out <- &StorageDealLsResult{
					Deal: storageDeal,
				}
			}
		}
		close(out)
	}()

	return out, nil
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
