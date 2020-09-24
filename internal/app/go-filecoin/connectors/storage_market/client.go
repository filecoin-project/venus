package storagemarketconnector

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-state-types/exitcode"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
	"reflect"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	spaminer "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	spapow "github.com/filecoin-project/specs-actors/actors/builtin/power"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// StorageClientNodeConnector adapts the node to provide the correct interface to the storage client.
type StorageClientNodeConnector struct {
	connectorCommon

	clientAddr ClientAddressGetter
	cborStore  cbor.IpldStore
}

func (s *StorageClientNodeConnector) WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, cid.Cid, error) error) error {
	return s.waiter.Wait(ctx, mcid, msg.DefaultMessageWaitLookback, func(block *block.Block, msg *types.SignedMessage, recepit *vm.MessageReceipt) error {
		return onCompletion(recepit.ExitCode, recepit.ReturnValue, mcid, nil)
	})
}

func (s *StorageClientNodeConnector) DealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, isVerified bool) (abi.TokenAmount, abi.TokenAmount, error) {
	head := s.chainStore.Head()
	ts, err := s.chainStore.GetTipSet(head)
	if err != nil {
		return abi.TokenAmount{}, abi.TokenAmount{}, err
	}

	view := s.stateViewer.StateView(ts.At(0).StateRoot)
	bounds, err := view.MarketDealProviderCollateralBounds(ctx, size, isVerified, ts.At(0).Height)
	if err != nil {
		return abi.TokenAmount{}, abi.TokenAmount{}, err
	}
	return bounds.Min, bounds.Max, nil
}

func (s *StorageClientNodeConnector) OnDealExpiredOrSlashed(ctx context.Context, dealID abi.DealID, onDealExpired storagemarket.DealExpiredCallback, onDealSlashed storagemarket.DealSlashedCallback) error {
	panic("implement me")
}

func (s *StorageClientNodeConnector) GetMinerInfo(ctx context.Context, maddr address.Address, tok shared.TipSetToken) (*storagemarket.StorageProviderInfo, error) {
	tsk, err := s.getTipSet(tok)
	if err != nil {
		return nil, err
	}

	view := s.stateViewer.StateView(tsk.At(0).StateRoot)

	minerInfo, err := view.MinerInfo(ctx, maddr)
	if err != nil {
		return nil, err
	}
	multiaddrs := make([]ma.Multiaddr, 0, len(minerInfo.Multiaddrs))
	for _, a := range minerInfo.Multiaddrs {
		maddr, err := ma.NewMultiaddrBytes(a)
		if err != nil {
			return nil, err
		}
		multiaddrs = append(multiaddrs, maddr)
	}
	return &storagemarket.StorageProviderInfo{
		Address:    maddr,
		Owner:      minerInfo.Owner,
		Worker:     minerInfo.Worker,
		SectorSize: uint64(minerInfo.SectorSize),
		PeerID:     peer.ID(minerInfo.PeerId),
		Addrs:      multiaddrs,
	}, nil
}

func (s *StorageClientNodeConnector) getTipSet(tok shared.TipSetToken) (block.TipSet, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return block.TipSet{}, xerrors.Wrapf(err, "failed to marshal TipSetToken into a TipSetKey")
	}

	return s.chainStore.GetTipSet(tsk)
}

type ClientAddressGetter func() (address.Address, error)

var _ storagemarket.StorageClientNode = &StorageClientNodeConnector{}

// NewStorageClientNodeConnector creates a new connector
func NewStorageClientNodeConnector(
	cbor cbor.IpldStore,
	cs *cst.ChainStateReadWriter,
	w *msg.Waiter,
	s types.Signer,
	ob *message.Outbox,
	ca ClientAddressGetter,
	sv *appstate.Viewer,
) *StorageClientNodeConnector {
	return &StorageClientNodeConnector{
		connectorCommon: connectorCommon{cs, sv, w, s, ob},
		cborStore:       cbor,
		clientAddr:      ca,
	}
}

// AddFunds adds storage market funds for a storage client
func (s *StorageClientNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	clientAddr, err := s.clientAddr()
	if err != nil {
		return cid.Undef, err
	}
	return s.addFunds(ctx, clientAddr, addr, amount)
}

// EnsureFunds compares the passed amount to the available balance for an address, and will add funds if necessary
func (s *StorageClientNodeConnector) EnsureFunds(ctx context.Context, addr, walletAddr address.Address, amount abi.TokenAmount, tok shared.TipSetToken) (cid.Cid, error) {
	balance, err := s.GetBalance(ctx, addr, tok)
	if err != nil {
		return cid.Undef, err
	}

	if balance.Available.LessThan(amount) {
		return s.AddFunds(ctx, addr, big.Sub(amount, balance.Available))
	}

	return cid.Undef, err
}

// ListClientDeals returns all deals published on chain for the given account
func (s *StorageClientNodeConnector) ListClientDeals(ctx context.Context, addr address.Address, tok shared.TipSetToken) ([]storagemarket.StorageDeal, error) {
	return s.listDeals(ctx, tok, func(proposal *market.DealProposal, _ *market.DealState) bool {
		return proposal.Client == addr
	})
}

// ListStorageProviders finds all miners that will provide storage
func (s *StorageClientNodeConnector) ListStorageProviders(ctx context.Context, tok shared.TipSetToken) ([]*storagemarket.StorageProviderInfo, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return nil, xerrors.Wrapf(err, "failed to marshal TipSetToken into a TipSetKey")
	}

	var spState spapow.State
	err := s.chainStore.GetActorStateAt(ctx, tsk, builtin.StoragePowerActorAddr, &spState)
	if err != nil {
		return nil, err
	}

	infos := []*storagemarket.StorageProviderInfo{}
	powerHamt, err := hamt.LoadNode(ctx, s.cborStore, spState.Claims)
	if err != nil {
		return nil, err
	}

	err = powerHamt.ForEach(ctx, func(minerAddrStr string, _ interface{}) error {
		minerAddr, err := address.NewFromString(minerAddrStr)
		if err != nil {
			return err
		}

		var mState spaminer.State
		err = s.chainStore.GetActorStateAt(ctx, tsk, minerAddr, &mState)
		if err != nil {
			return err
		}

		viewer, err := s.chainStore.StateView(tsk)
		if err != nil {
			return err
		}
		info, err := viewer.MinerInfo(ctx, minerAddr)
		if err != nil {
			return err
		}
		infos = append(infos, &storagemarket.StorageProviderInfo{
			Address:    minerAddr,
			Owner:      info.Owner,
			Worker:     info.Worker,
			SectorSize: uint64(info.SectorSize),
			PeerID:     peer.ID(info.PeerId),
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
func (s *StorageClientNodeConnector) ValidatePublishedDeal(ctx context.Context, deal storagemarket.ClientDeal) (dealID abi.DealID, err error) {
	var unsigned types.UnsignedMessage
	var receipt *vm.MessageReceipt

	// TODO: This is an inefficient way to discover a deal ID. See if we can find it uniquely on chain some other way or store the dealID when the message first lands (#4066).
	// give the wait 30 seconds to avoid races
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Fetch receipt to return dealId
	about2Days := uint64(24 * 60)
	err = s.waiter.Wait(ctx, *deal.PublishMessage, about2Days, func(_ *block.Block, msg *types.SignedMessage, rcpt *vm.MessageReceipt) error {
		unsigned = msg.Message
		receipt = rcpt
		return nil
	})
	if err != nil {
		return 0, err
	}

	tok, err := encoding.Encode(s.chainStore.Head())
	if err != nil {
		return 0, err
	}

	minerWorker, err := s.GetMinerWorkerAddress(ctx, deal.Proposal.Provider, tok)
	if err != nil {
		return 0, err
	}

	if unsigned.From != minerWorker {
		return 0, xerrors.Errorf("deal wasn't published by storage provider: from=%s, provider=%s", unsigned.From, deal.Proposal.Provider)
	}

	if unsigned.To != builtin.StorageMarketActorAddr {
		return 0, xerrors.Errorf("deal publish message wasn't set to StorageMarket actor (to=%s)", unsigned.To)
	}

	if unsigned.Method != builtin.MethodsMarket.PublishStorageDeals {
		return 0, xerrors.Errorf("deal publish message called incorrect method (method=%d)", unsigned.Method)
	}

	var params market.PublishStorageDealsParams
	err = encoding.Decode(unsigned.Params, &params)
	if err != nil {
		return 0, err
	}

	msgProposals := params.Deals
	// The return value doesn't recapitulate the whole deal. If inspection is required, we should look up the deal
	// in the market actor state.

	for _, proposal := range msgProposals {
		if reflect.DeepEqual(proposal.Proposal, deal.Proposal) {
			var ret market.PublishStorageDealsReturn
			err := encoding.Decode(receipt.ReturnValue, &ret)
			if err != nil {
				return 0, err
			}
			return ret.IDs[0], nil
		}
	}

	return 0, xerrors.Errorf("published deal does not match ClientDeal")
}

// SignProposal uses the local wallet to sign the given proposal
func (s *StorageClientNodeConnector) SignProposal(ctx context.Context, signer address.Address, proposal market.DealProposal) (*market.ClientDealProposal, error) {
	buf, err := encoding.Encode(&proposal)
	if err != nil {
		return nil, err
	}

	signature, err := s.SignBytes(ctx, signer, buf)
	if err != nil {
		return nil, err
	}

	return &market.ClientDealProposal{
		Proposal:        proposal,
		ClientSignature: *signature,
	}, nil
}

// GetDefaultWalletAddress returns the default account for this node
func (s *StorageClientNodeConnector) GetDefaultWalletAddress(_ context.Context) (address.Address, error) {
	return s.clientAddr()
}

// ValidateAskSignature ensures the given ask has been signed correctly
func (s *StorageClientNodeConnector) ValidateAskSignature(ctx context.Context, signed *storagemarket.SignedStorageAsk, tok shared.TipSetToken) (bool, error) {
	ask := signed.Ask

	buf, err := encoding.Encode(ask)
	if err != nil {
		return false, err
	}

	return s.VerifySignature(ctx, *signed.Signature, ask.Miner, buf, tok)
}

// EventLogger logs new events on the storage client
func (s *StorageClientNodeConnector) EventLogger(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
	log.Infof("Event: %s, Proposal CID: %s, State: %s, Message: %s", storagemarket.ClientEvents[event], deal.ProposalCid, storagemarket.DealStates[deal.State], deal.Message)
}
