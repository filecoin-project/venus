package storagemarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	smtypes "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	fcsm "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type StorageClientNodeConnector struct {
	ConnectorCommon

	// need to init these
	clientAddr address.Address
	outbox     *message.Outbox
	waiter     *msg.Waiter
}

var _ storagemarket.StorageClientNode = &StorageClientNodeConnector{}

func NewStorageClientNodeConnector(
	cs chainReader,
	w *msg.Waiter,
	wlt *wallet.Wallet,
) *StorageClientNodeConnector {
	return &StorageClientNodeConnector{
		ConnectorCommon: ConnectorCommon{cs, w, wlt},
	}
}

// TODO: Combine this and the provider AddFunds into a method in ConnectorCommon?  We would need to pass in the "from" param
func (s *StorageClientNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	params, err := abi.ToEncodedValues(addr)
	if err != nil {
		return err
	}

	fromAddr, err := fcaddr.NewFromBytes(s.clientAddr.Bytes())
	if err != nil {
		return err
	}

	mcid, cerr, err := s.outbox.Send(
		ctx,
		fromAddr,
		fcaddr.StorageMarketAddress,
		types.NewAttoFIL(amount.Int),
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		fcsm.AddBalance,
		params,
	)
	if err != nil {
		return err
	}

	_, err = s.wait(ctx, mcid, cerr)

	return err
}

func (s *StorageClientNodeConnector) EnsureFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	// State query
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ListClientDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	// State query
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ListStorageProviders(ctx context.Context) ([]*storagemarket.StorageProviderInfo, error) {
	// State query
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ValidatePublishedDeal(ctx context.Context, deal storagemarket.ClientDeal) (uint64, error) {
	// State query
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) SignProposal(ctx context.Context, signer address.Address, proposal *storagemarket.StorageDealProposal) error {
	signFn := func(ctx context.Context, data []byte) (*smtypes.Signature, error) {
		return s.SignBytes(ctx, signer, data)
	}

	return proposal.Sign(ctx, signFn)
}

func (s *StorageClientNodeConnector) GetDefaultWalletAddress(ctx context.Context) (address.Address, error) {
	return s.clientAddr, nil
}

func (s *StorageClientNodeConnector) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealId uint64, cb storagemarket.DealSectorCommittedCallback) error {
	panic("TODO: go-fil-markets integration")
}

func (s *StorageClientNodeConnector) ValidateAskSignature(ask *smtypes.SignedStorageAsk) error {
	panic("TODO: go-fil-markets integration")
}
