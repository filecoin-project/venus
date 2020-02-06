package storagemarketconnector

import (
	"bytes"
	"context"

	"github.com/ipfs/go-cid"

	smcborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/ipfs/go-hamt-ipld"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	fcsm "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	spaminer "github.com/filecoin-project/specs-actors/actors/builtin/storage_miner"
	spapow "github.com/filecoin-project/specs-actors/actors/builtin/storage_power"

	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"
	smtypes "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

// StorageClientNodeConnector adapts the node to provide the correct interface to the storage client.
type StorageClientNodeConnector struct {
	connectorCommon

	clientAddr address.Address
	cborStore  hamt.CborIpldStore
}

var _ storagemarket.StorageClientNode = &StorageClientNodeConnector{}

// NewStorageClientNodeConnector creates a new connector
func NewStorageClientNodeConnector(
	cbor hamt.CborIpldStore,
	cs chainReader,
	w *msg.Waiter,
	wlt *wallet.Wallet,
	ob *message.Outbox,
	ca address.Address,
	wg WorkerGetter,
) *StorageClientNodeConnector {
	return &StorageClientNodeConnector{
		connectorCommon: connectorCommon{cs, w, wlt, ob, wg},
		cborStore:       cbor,
		clientAddr:      ca,
	}
}

// AddFunds sends a message to add collateral for the given address
func (s *StorageClientNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount tokenamount.TokenAmount) error {
	return s.addFunds(ctx, s.clientAddr, addr, amount)
}

// EnsureFunds checks the current balance for an address and adds funds if the balance is below the given amount
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

// ListClientDeals returns all deals published on chain for the given account
func (s *StorageClientNodeConnector) ListClientDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	return s.listDeals(ctx, addr)
}

// ListStorageProviders finds all miners that will provide storage
func (s *StorageClientNodeConnector) ListStorageProviders(ctx context.Context) ([]*storagemarket.StorageProviderInfo, error) {
	head := s.chainStore.Head()
	var spState spapow.StoragePowerActorState
	err := s.chainStore.GetActorStateAt(ctx, head, vmaddr.StoragePowerAddress, &spState)
	if err != nil {
		return nil, err
	}

	infos := []*storagemarket.StorageProviderInfo{}
	powerHamt, err := hamt.LoadNode(ctx, s.cborStore, spState.PowerTable)
	if err != nil {
		return nil, err
	}

	err = powerHamt.ForEach(ctx, func(minerAddrStr string, _ interface{}) error {
		minerAddr, err := address.NewFromString(minerAddrStr)
		if err != nil {
			return err
		}

		var mState spaminer.StorageMinerActorState
		err = s.chainStore.GetActorStateAt(ctx, head, minerAddr, &mState)
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

// ValidatePublishedDeal validates a deal has been published correctly
// Adapted from https://github.com/filecoin-project/lotus/blob/3b34eba6124d16162b712e971f0db2ee108e0f67/markets/storageadapter/client.go#L156
func (s *StorageClientNodeConnector) ValidatePublishedDeal(ctx context.Context, deal storagemarket.ClientDeal) (uint64, error) {
	// Fetch receipt to return dealId
	chnMsg, found, err := s.waiter.Find(ctx, func(msg *types.SignedMessage, c cid.Cid) bool {
		return c.Equals(*deal.PublishMessage)
	})
	if err != nil {
		return 0, err
	}

	if !found {
		return 0, xerrors.Errorf("Could not find published deal message %s", deal.PublishMessage.String())
	}

	unsigned := chnMsg.Message.Message

	minerWorker, err := s.GetMinerWorker(ctx, deal.Proposal.Provider)
	if err != nil {
		return 0, err
	}

	if unsigned.From != minerWorker {
		return 0, xerrors.Errorf("deal wasn't published by storage provider: from=%s, provider=%s", unsigned.From, deal.Proposal.Provider)
	}

	if unsigned.To != vmaddr.StorageMarketAddress {
		return 0, xerrors.Errorf("deal publish message wasn't set to StorageMarket actor (to=%s)", unsigned.To)
	}

	if unsigned.Method != fcsm.PublishStorageDeals {
		return 0, xerrors.Errorf("deal publish message called incorrect method (method=%s)", unsigned.Method)
	}

	values, err := abi.DecodeValues(unsigned.Params, []abi.Type{abi.StorageDealProposals})
	if err != nil {
		return 0, err
	}

	msgProposals := values[0].Val.([]types.StorageDealProposal)

	proposal := msgProposals[0] // TODO: Support more than one deal

	// TODO: find a better way to do this
	equals := bytes.Equal(proposal.PieceRef, deal.Proposal.PieceRef) &&
		uint64(proposal.PieceSize) == deal.Proposal.PieceSize &&
		//proposal.Client == deal.Proposal.Client &&
		//proposal.Provider == deal.Proposal.Provider &&
		uint64(proposal.ProposalExpiration) == deal.Proposal.ProposalExpiration &&
		uint64(proposal.Duration) == deal.Proposal.Duration &&
		uint64(proposal.StoragePricePerEpoch) == deal.Proposal.StoragePricePerEpoch.Uint64() &&
		uint64(proposal.StorageCollateral) == deal.Proposal.StorageCollateral.Uint64() &&
		bytes.Equal([]byte(*proposal.ProposerSignature), deal.Proposal.ProposerSignature.Data)

	if equals {
		sectorIDVal, err := abi.Deserialize(chnMsg.Receipt.Return[0], abi.SectorID)
		if err != nil {
			return 0, err
		}

		sectorID, ok := sectorIDVal.Val.(uint64)
		if !ok {
			return 0, xerrors.New("publish deal return is not a sector ID")
		}
		return sectorID, nil
	}

	return 0, xerrors.Errorf("published deal does not match ClientDeal")
}

// SignProposal uses the local wallet to sign the given proposal
func (s *StorageClientNodeConnector) SignProposal(ctx context.Context, signer address.Address, proposal *storagemarket.StorageDealProposal) error {
	signFn := func(ctx context.Context, data []byte) (*smtypes.Signature, error) {
		return s.SignBytes(ctx, signer, data)
	}

	return proposal.Sign(ctx, signFn)
}

// GetDefaultWalletAddress returns the default account for this node
func (s *StorageClientNodeConnector) GetDefaultWalletAddress(ctx context.Context) (address.Address, error) {
	return s.clientAddr, nil
}

// ValidateAskSignature ensures the given ask has been signed correctly
func (s *StorageClientNodeConnector) ValidateAskSignature(signed *smtypes.SignedStorageAsk) error {
	ask := signed.Ask
	data, err := smcborutil.Dump(ask)
	if err != nil {
		return err
	}

	if types.IsValidSignature(data, ask.Miner, signed.Signature.Data) {
		return nil
	}

	return xerrors.Errorf("invalid ask signature")
}
