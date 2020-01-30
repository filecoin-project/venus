package storagemarketconnector

import (
	"context"

	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-address"
	spaminer "github.com/filecoin-project/specs-actors/actors/builtin/storage_miner"
	spapow "github.com/filecoin-project/specs-actors/actors/builtin/storage_power"

	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	smtypes "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type StorageClientNodeConnector struct {
	ConnectorCommon

	// need to init these
	clientAddr address.Address
	cborStore  hamt.CborIpldStore
}

var _ storagemarket.StorageClientNode = &StorageClientNodeConnector{}

func NewStorageClientNodeConnector(
	cbor hamt.CborIpldStore,
	cs chainReader,
	w *msg.Waiter,
	wlt *wallet.Wallet,
	ob *message.Outbox,
	ca address.Address,
) *StorageClientNodeConnector {
	return &StorageClientNodeConnector{
		ConnectorCommon: ConnectorCommon{cs, w, wlt, ob},
		cborStore:       cbor,
		clientAddr:      ca,
	}
}

func (s *StorageClientNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	return s.addFunds(ctx, s.clientAddr, addr, amount)
}

func (s *StorageClientNodeConnector) EnsureFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	balance, err := s.GetBalance(ctx, addr)
	if err != nil {
		return err
	}

	if !balance.Available.LessThan(amount) {
		return nil
	}

	return s.AddFunds(ctx, addr, tokenamount.Sub(amount, balance.Available))
}

func (s *StorageClientNodeConnector) ListClientDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	return s.listDeals(ctx, addr)
}

func (s *StorageClientNodeConnector) ListStorageProviders(ctx context.Context) ([]*storagemarket.StorageProviderInfo, error) {
	head := s.chainStore.Head()
	var spState spapow.StoragePowerActorState
	err := s.chainStore.GetActorStateAt(ctx, head, fcaddr.StoragePowerAddress, &spState)
	if err != nil {
		return nil, err
	}

	infos := []*storagemarket.StorageProviderInfo{}
	powerHamt, err := hamt.LoadNode(ctx, s.cborStore, spState.PowerTable)
	err = powerHamt.ForEach(ctx, func(minerAddrStr string, _ interface{}) error {
		fcMinerAddr, err := fcaddr.NewFromString(minerAddrStr)
		if err != nil {
			return err
		}

		var mState spaminer.StorageMinerActorState
		err = s.chainStore.GetActorStateAt(ctx, head, fcMinerAddr, &mState)
		if err != nil {
			return err
		}

		minerAddr, err := address.NewFromString(minerAddrStr)
		if err != nil {
			return err
		}

		info := mState.Info
		infos = append(infos, &storagemarket.StorageProviderInfo{
			Address:    minerAddr,
			Owner:      info.Owner,
			Worker:     info.Worker,
			SectorSize: uint64(info.SectorSize),
			PeerID:     info.PeerId,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return infos, nil
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
