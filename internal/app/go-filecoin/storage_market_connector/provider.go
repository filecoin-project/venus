package storagemarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	t2 "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
)

type StorageProviderNodeConnector struct{}

func NewStorageProviderNodeConnector() *StorageProviderNodeConnector {
	return &StorageProviderNodeConnector{}
}

func (s *StorageProviderNodeConnector) MostRecentStateId(ctx context.Context) (storagemarket.StateKey, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) EnsureFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) GetBalance(ctx context.Context, addr address.Address) (storagemarket.Balance, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) PublishDeals(ctx context.Context, deal storagemarket.MinerDeal) (storagemarket.DealID, cid.Cid, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) ListProviderDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) OnDealComplete(ctx context.Context, deal storagemarket.MinerDeal, pieceSize uint64, pieceReader io.Reader) (uint64, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) SignBytes(ctx context.Context, signer address.Address, b []byte) (*t2.Signature, error) {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID uint64, cb storagemarket.DealSectorCommittedCallback) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageProviderNodeConnector) LocatePieceForDealWithinSector(ctx context.Context, dealID uint64) (sectorID uint64, offset uint64, length uint64, err error) {
	panic("TODO: go-fil-markets integration")
}
