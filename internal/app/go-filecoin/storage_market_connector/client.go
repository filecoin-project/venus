package storagemarketconnector

import (
	"context"

	a2 "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	"github.com/filecoin-project/go-fil-markets/shared/types"
	m "github.com/filecoin-project/go-fil-markets/storagemarket"
)

type StorageClientNodeConnector struct{}

func NewStorageClientNodeConnector() *StorageClientNodeConnector {
	return &StorageClientNodeConnector{}
}

func (s *StorageClientNodeConnector) MostRecentStateId(ctx context.Context) (m.StateKey, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) AddFunds(ctx context.Context, addr a2.Address, amount tokenamount.TokenAmount) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) EnsureFunds(ctx context.Context, addr a2.Address, amount tokenamount.TokenAmount) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) GetBalance(ctx context.Context, addr a2.Address) (m.Balance, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ListClientDeals(ctx context.Context, addr a2.Address) ([]m.StorageDeal, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ListStorageProviders(ctx context.Context) ([]*m.StorageProviderInfo, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ValidatePublishedDeal(ctx context.Context, deal m.ClientDeal) (uint64, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) SignProposal(ctx context.Context, signer a2.Address, proposal *m.StorageDealProposal) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) GetDefaultWalletAddress(ctx context.Context) (a2.Address, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) OnDealSectorCommitted(ctx context.Context, provider a2.Address, dealId uint64, cb m.DealSectorCommittedCallback) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ValidateAskSignature(ask *types.SignedStorageAsk) error {
	panic("TODO: go-fil-markets integration")
}
