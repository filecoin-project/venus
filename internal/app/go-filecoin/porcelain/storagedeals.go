package porcelain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore/query"
	errors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
)

var (
	// ErrDealNotFound means DealGet failed to find a matching deal
	ErrDealNotFound = errors.New("deal not found")
)

// StorageDealLsResult represents a result from the storage deal Ls method. This
// can either be an error or a storage deal.
type StorageDealLsResult struct {
	Deal storagedeal.Deal
	Err  error
}

type dealGetPlumbing interface {
	DealsLs(context.Context) (<-chan *StorageDealLsResult, error)
}

// DealGet returns a single deal matching a given cid or an error
func DealGet(ctx context.Context, plumbing dealGetPlumbing, dealCid cid.Cid) (*storagedeal.Deal, error) {
	dealCh, err := plumbing.DealsLs(ctx)
	if err != nil {
		return nil, err
	}
	for deal := range dealCh {
		if deal.Err != nil {
			return nil, deal.Err
		}
		if deal.Deal.Response.ProposalCid == dealCid {
			return &deal.Deal, nil
		}
	}
	return nil, ErrDealNotFound
}

type dealLsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	DealsIterator() (*query.Results, error)
}

// DealsLs returns an channel with all deals or a possible error
func DealsLs(ctx context.Context, plumbing dealLsPlumbing) (<-chan *StorageDealLsResult, error) {
	out := make(chan *StorageDealLsResult)
	results, err := plumbing.DealsIterator()
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(out)
		for entry := range (*results).Next() {
			select {
			case <-ctx.Done():
				out <- &StorageDealLsResult{
					Err: ctx.Err(),
				}
				return
			default:
				var storageDeal storagedeal.Deal
				if err := encoding.Decode(entry.Value, &storageDeal); err != nil {
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
	}()

	return out, nil
}
