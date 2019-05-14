package porcelain

import (
	"github.com/ipfs/go-cid"
	errors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
)

var (
	// ErrDealNotFound means DealGet failed to find a matching deal
	ErrDealNotFound = errors.New("deal not found")
)

type strgdlsPlumbing interface {
	DealsLs() ([]*storagedeal.Deal, error)
}

// DealGet returns a single deal matching a given cid or an error
func DealGet(plumbing strgdlsPlumbing, dealCid cid.Cid) (*storagedeal.Deal, error) {
	deals, err := plumbing.DealsLs()
	if err != nil {
		return nil, err
	}
	for _, storageDeal := range deals {
		if storageDeal.Response.ProposalCid == dealCid {
			return storageDeal, nil
		}
	}
	return nil, ErrDealNotFound
}
