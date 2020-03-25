package storage

import (
	"context"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

// API is the storage API for the test environment
type API struct {
	storage      storagemarket.StorageClient
	piecemanager piecemanager.PieceManager
}

// NewAPI creates a new API
func NewAPI(s storagemarket.StorageClient, pm piecemanager.PieceManager) *API {
	return &API{s, pm}
}

// PledgeSector creates a new, empty sector and seals it.
func (api *API) PledgeSector(ctx context.Context) error {
	return api.piecemanager.PledgeSector(ctx)
}
