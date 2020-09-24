package storagemarketconnector

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	spaminer "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// StorageProviderNodeConnector adapts the node to provide an interface for the storage provider
type StorageProviderNodeConnector struct {
	connectorCommon

	minerAddr    address.Address
	chainStore   chainReader
	outbox       *message.Outbox
	pieceManager piecemanager.PieceManager
}

var _ storagemarket.StorageProviderNode = &StorageProviderNodeConnector{}

// NewStorageProviderNodeConnector creates a new connector
func NewStorageProviderNodeConnector(ma address.Address,
	cs *cst.ChainStateReadWriter,
	ob *message.Outbox,
	w *msg.Waiter,
	pm piecemanager.PieceManager,
	s types.Signer,
	sv *appstate.Viewer,
) *StorageProviderNodeConnector {
	return &StorageProviderNodeConnector{
		connectorCommon: connectorCommon{cs, sv, w, s, ob},
		chainStore:      cs,
		minerAddr:       ma,
		outbox:          ob,
		pieceManager:    pm,
	}
}

func (s *StorageProviderNodeConnector) WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, cid.Cid, error) error) error {
	return s.waiter.Wait(ctx, mcid, msg.DefaultMessageWaitLookback, func(block *block.Block, msg *types.SignedMessage, recepit *vm.MessageReceipt) error {
		return onCompletion(recepit.ExitCode, recepit.ReturnValue, mcid, nil)
	})
}

func (s *StorageProviderNodeConnector) DealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, isVerified bool) (abi.TokenAmount, abi.TokenAmount, error) {
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

func (s *StorageProviderNodeConnector) OnDealExpiredOrSlashed(ctx context.Context, dealID abi.DealID, onDealExpired storagemarket.DealExpiredCallback, onDealSlashed storagemarket.DealSlashedCallback) error {
	panic("implement me")
	/*head := s.chainStore.Head()
	ts, err := s.chainStore.GetTipSet(head)
	if err != nil {
		return err
	}

	view := s.stateViewer.StateView(ts.At(0).StateRoot)
	sd, err := view.StateMarketStorageDeal(ctx, dealID)
	if err != nil {
		return xerrors.Errorf("client: failed to look up deal %d on chain: %w", dealID, err)
	}

	// Called immediately to check if the deal has already expired or been slashed
	checkFunc := func(ts *block.TipSet) (done bool, more bool, err error) {
		if ts == nil {
			// keep listening for events
			return false, true, nil
		}

		// Check if the deal has already expired
		if sd.Proposal.EndEpoch <= ts.Height() {
			onDealExpired(nil)
			return true, false, nil
		}

		// If there is no deal assume it's already been slashed
		if sd.State.SectorStartEpoch < 0 {
			onDealSlashed(ts.Height(), nil)
			return true, false, nil
		}

		// No events have occurred yet, so return
		// done: false, more: true (keep listening for events)
		return false, true, nil
	}

	// Called when there was a match against the state change we're looking for
	// and the chain has advanced to the confidence height
	stateChanged := func(ts *types.TipSet, ts2 *types.TipSet, states events.StateChange, h abi.ChainEpoch) (more bool, err error) {
		// Check if the deal has already expired
		if sd.Proposal.EndEpoch <= ts2.Height() {
			onDealExpired(nil)
			return false, nil
		}

		// Timeout waiting for state change
		if states == nil {
			log.Error("timed out waiting for deal expiry")
			return false, nil
		}

		changedDeals, ok := states.(state.ChangedDeals)
		if !ok {
			panic("Expected state.ChangedDeals")
		}

		deal, ok := changedDeals[dealID]
		if !ok {
			// No change to deal
			return true, nil
		}

		// Deal was slashed
		if deal.To == nil {
			onDealSlashed(ts2.Height(), nil)
			return false, nil
		}

		return true, nil
	}

	// Called when there was a chain reorg and the state change was reverted
	revert := func(ctx context.Context, ts *types.TipSet) error {
		// TODO: Is it ok to just ignore this?
		log.Warn("deal state reverted; TODO: actually handle this!")
		return nil
	}

	// Watch for state changes to the deal
	preds := state.NewStatePredicates(n)
	dealDiff := preds.OnStorageMarketActorChanged(
		preds.OnDealStateChanged(
			preds.DealStateChangedForIDs([]abi.DealID{dealID})))
	match := func(oldTs, newTs *block.TipSet) (bool, events.StateChange, error) {
		return dealDiff(ctx, oldTs.Key(), newTs.Key())
	}

	// Wait until after the end epoch for the deal and then timeout
	timeout := (sd.Proposal.EndEpoch - head.Height()) + 1
	if err := n.ev.StateChanged(checkFunc, stateChanged, revert, int(build.MessageConfidence)+1, timeout, match); err != nil {
		return xerrors.Errorf("failed to set up state changed handler: %w", err)
	}

	return nil*/
}

func (s *StorageProviderNodeConnector) GetDataCap(ctx context.Context, addr address.Address, tok shared.TipSetToken) (*verifreg.DataCap, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return nil, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}
	ts, err := s.chainStore.GetTipSet(tsk)
	if err != nil {
		return nil, err
	}

	view := s.stateViewer.StateView(ts.At(0).StateRoot)
	return view.StateVerifiedClientStatus(ctx, addr)
}

// AddFunds adds storage market funds for a storage provider
func (s *StorageProviderNodeConnector) AddFunds(ctx context.Context, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	tok, err := encoding.Encode(s.chainStore.Head())
	if err != nil {
		return cid.Undef, err
	}

	workerAddr, err := s.GetMinerWorkerAddress(ctx, s.minerAddr, tok)
	if err != nil {
		return cid.Undef, err
	}

	return s.addFunds(ctx, workerAddr, addr, amount)
}

// EnsureFunds compares the passed amount to the available balance for an address, and will add funds if necessary
func (s *StorageProviderNodeConnector) EnsureFunds(ctx context.Context, addr, walletAddr address.Address, amount abi.TokenAmount, tok shared.TipSetToken) (cid.Cid, error) {
	balance, err := s.GetBalance(ctx, addr, tok)
	if err != nil {
		return cid.Undef, err
	}

	if balance.Available.LessThan(amount) {
		return s.AddFunds(ctx, addr, big.Sub(amount, balance.Available))
	}

	return cid.Undef, err
}

// PublishDeals publishes storage deals on chain
func (s *StorageProviderNodeConnector) PublishDeals(ctx context.Context, deal storagemarket.MinerDeal) (cid.Cid, error) {
	params := market.PublishStorageDealsParams{Deals: []market.ClientDealProposal{deal.ClientDealProposal}}

	tok, err := encoding.Encode(s.chainStore.Head())
	if err != nil {
		return cid.Undef, err
	}

	workerAddr, err := s.GetMinerWorkerAddress(ctx, s.minerAddr, tok)
	if err != nil {
		return cid.Undef, err
	}

	mcid, _, err := s.outbox.Send(
		ctx,
		workerAddr,
		builtin.StorageMarketActorAddr,
		types.ZeroAttoFIL,
		types.NewGasFeeCap(0),
		types.NewGasPremium(0),
		gas.NewGas(10000),
		true,
		builtin.MethodsMarket.PublishStorageDeals,
		&params,
	)

	if err != nil {
		return cid.Undef, err
	}

	return mcid, err
}

// ListProviderDeals lists all deals for the given provider
func (s *StorageProviderNodeConnector) ListProviderDeals(ctx context.Context, addr address.Address, tok shared.TipSetToken) ([]storagemarket.StorageDeal, error) {
	return s.listDeals(ctx, tok, func(proposal *market.DealProposal, dealState *market.DealState) bool {
		return proposal.Provider == addr
	})
}

// OnDealComplete adds the piece to the storage provider
func (s *StorageProviderNodeConnector) OnDealComplete(ctx context.Context, deal storagemarket.MinerDeal, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) (*storagemarket.PackingResult, error) {
	// TODO: callback.
	return s.pieceManager.SealPieceIntoNewSector(ctx, deal.DealID, deal.Proposal.StartEpoch, deal.Proposal.EndEpoch, pieceSize, pieceReader)
}

// LocatePieceForDealWithinSector finds the sector, offset and length of a piece associated with the given deal id
func (s *StorageProviderNodeConnector) LocatePieceForDealWithinSector(ctx context.Context, dealID abi.DealID, tok shared.TipSetToken) (sectorNumber abi.SectorNumber, offset abi.PaddedPieceSize, length abi.PaddedPieceSize, err error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return 0, 0, 0, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}

	var smState market.State
	err = s.chainStore.GetActorStateAt(ctx, tsk, builtin.StorageMarketActorAddr, &smState)
	if err != nil {
		return 0, 0, 0, err
	}

	stateStore := state.StoreFromCbor(ctx, s.chainStore)
	proposals, err := adt.AsArray(stateStore, smState.Proposals)
	if err != nil {
		return 0, 0, 0, err
	}

	var minerState spaminer.State
	err = s.chainStore.GetActorStateAt(ctx, tsk, s.minerAddr, &minerState)
	if err != nil {
		return 0, 0, 0, err
	}

	precommitted, err := adt.AsMap(stateStore, minerState.PreCommittedSectors)
	if err != nil {
		return 0, 0, 0, err
	}

	var sectorInfo spaminer.SectorPreCommitOnChainInfo
	err = precommitted.ForEach(&sectorInfo, func(key string) error {
		k, err := abi.ParseIntKey(key)
		if err != nil {
			return err
		}
		sectorNumber = abi.SectorNumber(k)

		for _, deal := range sectorInfo.Info.DealIDs {
			if deal == dealID {
				offset = abi.PaddedPieceSize(0)
				for _, did := range sectorInfo.Info.DealIDs {
					var proposal market.DealProposal
					found, err := proposals.Get(uint64(did), &proposal)
					if err != nil {
						return err
					}
					if !found {
						return errors.Errorf("Could not find miner deal %d in storage market state", did)
					}

					if did == dealID {
						sectorNumber = abi.SectorNumber(k)
						length = abi.PaddedPieceSize(proposal.PieceSize)
						return nil // Found!
					}
					offset += abi.PaddedPieceSize(proposal.PieceSize)
				}
			}
		}
		return errors.New("Deal not found")
	})
	return
}

// EventLogger logs new events on the storage provider
func (s *StorageProviderNodeConnector) EventLogger(event storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
	log.Infof("Event: %s, Proposal CID: %s, State: %s, Message: %s", storagemarket.ProviderEvents[event], deal.ProposalCid, storagemarket.DealStates[deal.State], deal.Message)
}
