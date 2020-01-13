package storage

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

const (
	_ = iota
	// ErrDuplicateDeal indicates that a deal being proposed is a duplicate of an existing deal
	ErrDuplicateDeal
)

// Errors map error codes to messages
var Errors = map[uint8]error{
	ErrDuplicateDeal: errors.New("proposal is a duplicate of existing deal; if you would like to create a duplicate, add the --allow-duplicates flag"),
}

const (
	// VoucherInterval defines how many block pass before creating a new voucher
	VoucherInterval = 1000
)

// Client is...
type Client struct {
}

// NewClient is...
func NewClient() *Client {
	panic("replace with storage client in go-fil-markets")

	return &Client{} // nolint:govet
}

// ProposeDeal proposes a storage deal to a miner.  Pass allowDuplicates = true to
// allow duplicate proposals without error.
func (smc *Client) ProposeDeal(ctx context.Context, miner address.Address, data cid.Cid, askID uint64, duration uint64, allowDuplicates bool) (*storagedeal.SignedResponse, error) {
	panic("replace with storage client in go-fil-markets")

	return nil, nil // nolint:govet
}
