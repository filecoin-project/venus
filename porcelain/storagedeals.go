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

type dealGetPlumbing interface {
	DealsLs() ([]*storagedeal.Deal, error)
}

// DealGet returns a single deal matching a given cid or an error
func DealGet(plumbing dealGetPlumbing, dealCid cid.Cid) (*storagedeal.Deal, error) {
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

type dealClientLsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	DealsLs() ([]*storagedeal.Deal, error)
}

// DealClientLs returns a slice of deals placed as a client
func DealClientLs(plumbing dealClientLsPlumbing) ([]*storagedeal.Deal, error) {
	var results []*storagedeal.Deal

	minerAddress, _ := plumbing.ConfigGet("mining.minerAddress")

	deals, err := plumbing.DealsLs()
	if err != nil {
		return results, err
	}

	for _, deal := range deals {
		if deal.Miner != minerAddress {
			results = append(results, deal)
		}
	}

	return results, nil
}

type dealMinerLsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	DealsLs() ([]*storagedeal.Deal, error)
}

// DealMinerLs returns a slice of deals received as a miner
func DealMinerLs(plumbing dealMinerLsPlumbing) ([]*storagedeal.Deal, error) {
	var results []*storagedeal.Deal

	minerAddress, _ := plumbing.ConfigGet("mining.minerAddress")

	deals, err := plumbing.DealsLs()
	if err != nil {
		return results, err
	}

	for _, deal := range deals {
		if deal.Miner == minerAddress {
			results = append(results, deal)
		}
	}

	return results, nil
}
