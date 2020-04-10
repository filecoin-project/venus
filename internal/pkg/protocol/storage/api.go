package storage

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

type storage interface {
	Client() storagemarket.StorageClient
	Provider() storagemarket.StorageProvider
	PieceManager() piecemanager.PieceManager
}

// API is the storage API for the test environment
type API struct {
	storage storage
}

// NewAPI creates a new API
func NewAPI(storage storage) *API {
	return &API{storage}
}

// PledgeSector creates a new, empty sector and seals it.
func (api *API) PledgeSector(ctx context.Context) error {
	pm := api.storage.PieceManager()
	if pm == nil {
		return errors.New("Piece manager not initialized")
	}

	return pm.PledgeSector(ctx)
}

// AddAsk stores a new price for storage
func (api *API) AddAsk(price abi.TokenAmount, duration abi.ChainEpoch) error {
	provider := api.storage.Provider()
	if provider == nil {
		return errors.New("Storage provider not initialized")
	}

	return provider.AddAsk(price, duration)
}

// ListAsks lists all asks for the miner
func (api *API) ListAsks(maddr address.Address) ([]*storagemarket.SignedStorageAsk, error) {
	provider := api.storage.Provider()
	if provider == nil {
		return nil, errors.New("Storage provider not initialized")
	}

	return provider.ListAsks(maddr), nil
}

// ProposeStorageDeal proposes a storage deal
func (api *API) ProposeStorageDeal(
	ctx context.Context,
	addr address.Address,
	info *storagemarket.StorageProviderInfo,
	data *storagemarket.DataRef,
	startEpoch abi.ChainEpoch,
	endEpoch abi.ChainEpoch,
	price abi.TokenAmount,
	collateral abi.TokenAmount,
	rt abi.RegisteredProof,
) (*storagemarket.ProposeStorageDealResult, error) {
	return api.storage.Client().ProposeStorageDeal(ctx, addr, info, data, startEpoch, endEpoch, price, collateral, rt)
}
