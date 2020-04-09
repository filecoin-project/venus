package storage

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// API is the storage API for the test environment
type API struct {
	storageClient   storagemarket.StorageClient
	storageProvider storagemarket.StorageProvider

	piecemanager piecemanager.PieceManager
}

// NewAPI creates a new API
func NewAPI(s storagemarket.StorageClient, sp storagemarket.StorageProvider, pm piecemanager.PieceManager) *API {
	return &API{s, sp, pm}
}

// PledgeSector creates a new, empty sector and seals it.
func (api *API) PledgeSector(ctx context.Context) error {
	return api.piecemanager.PledgeSector(ctx)
}

// AddAsk stores a new price for storage
func (api *API) AddAsk(price abi.TokenAmount, duration abi.ChainEpoch) error {
	return api.storageProvider.AddAsk(price, duration)
}

// ListAsks lists all asks for the miner
func (api *API) ListAsks(maddr address.Address) []*storagemarket.SignedStorageAsk {
	return api.storageProvider.ListAsks(maddr)
}
