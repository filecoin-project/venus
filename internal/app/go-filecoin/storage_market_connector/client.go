package storagemarketconnector

import (
	"bytes"
	"context"
	"time"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-hamt-ipld"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
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
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type StorageClientNodeConnector struct {
	ConnectorCommon

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
	wg WorkerGetter,
) *StorageClientNodeConnector {
	return &StorageClientNodeConnector{
		ConnectorCommon: ConnectorCommon{cs, w, wlt, ob, wg},
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

// Adapted from https://github.com/filecoin-project/lotus/blob/3b34eba6124d16162b712e971f0db2ee108e0f67/markets/storageadapter/client.go#L156
func (s *StorageClientNodeConnector) ValidatePublishedDeal(ctx context.Context, deal storagemarket.ClientDeal) (uint64, error) {
	// Fetch receipt to return dealId
	waitCtx, _ := context.WithTimeout(ctx, 10*time.Millisecond)
	var publishMsg *types.SignedMessage
	var publishReceipt *types.MessageReceipt

	err := s.waiter.Wait(waitCtx, *deal.PublishMessage, func(block *block.Block, signedMessage *types.SignedMessage, receipt *types.MessageReceipt) error {
		publishMsg = signedMessage
		publishReceipt = receipt
		return nil
	})

	if err != nil {
		return 0, err
	}

	unsigned := publishMsg.Message

	minerWorker, err := s.getFCWorker(ctx, deal.Proposal.Provider)
	if err != nil {
		return 0, err
	}

	if unsigned.From != minerWorker {
		return 0, xerrors.Errorf("deal wasn't published by storage provider: from=%s, provider=%s", unsigned.From, deal.Proposal.Provider)
	}

	if unsigned.To != fcaddr.StorageMarketAddress {
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
		return leb128.ToUInt64(publishReceipt.Return[0]), nil // TODO: Is this correct?
	}

	return 0, xerrors.Errorf("published deal does not match ClientDeal")
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

func (s *StorageClientNodeConnector) ValidateAskSignature(signed *smtypes.SignedStorageAsk) error {
	ask := signed.Ask
	data, err := cborutil.Dump(ask)
	if err != nil {
		return err
	}

	fcMiner, err := fcaddr.NewFromBytes(ask.Miner.Bytes())
	if err != nil {
		return err
	}

	if types.IsValidSignature(data, fcMiner, signed.Signature.Data) {
		return nil
	}

	return xerrors.Errorf("invalid ask signature")
}
