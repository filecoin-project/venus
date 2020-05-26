package storage

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

type storage interface {
	Client() storagemarket.StorageClient
	Provider() (storagemarket.StorageProvider, error)
	PieceManager() (piecemanager.PieceManager, error)
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
	pm, err := api.storage.PieceManager()
	if err != nil {
		return err
	}

	return pm.PledgeSector(ctx)
}

// AddAsk stores a new price for storage
func (api *API) AddAsk(price abi.TokenAmount, duration abi.ChainEpoch) error {
	provider, err := api.storage.Provider()
	if err != nil {
		return err
	}

	return provider.AddAsk(price, duration)
}

// ListAsks lists all asks for the miner
func (api *API) ListAsks(maddr address.Address) ([]*storagemarket.SignedStorageAsk, error) {
	provider, err := api.storage.Provider()
	if err != nil {
		return nil, err
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

// GetStorageDeal retrieves information about an in-progress deal
func (api *API) GetStorageDeal(ctx context.Context, c cid.Cid) (storagemarket.ClientDeal, error) {
	return api.storage.Client().GetLocalDeal(ctx, c)
}

// GetClientDeals retrieves information about a in-progress deals on th miner side
func (api *API) GetClientDeals(ctx context.Context) ([]storagemarket.ClientDeal, error) {
	return api.storage.Client().ListLocalDeals(ctx)
}

// GetProviderDeals retrieves information about a in-progress deals on th miner side
func (api *API) GetProviderDeals(ctx context.Context) ([]storagemarket.MinerDeal, error) {
	provider, err := api.storage.Provider()
	if err != nil {
		return nil, err
	}
	return provider.ListLocalDeals()
}
