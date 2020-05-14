package storagemarketconnector

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
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
	cs chainReader,
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
		types.NewGasPrice(1),
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
func (s *StorageProviderNodeConnector) OnDealComplete(ctx context.Context, deal storagemarket.MinerDeal, pieceSize abi.UnpaddedPieceSize, pieceReader io.Reader) error {
	// TODO: callback.
	return s.pieceManager.SealPieceIntoNewSector(ctx, deal.DealID, deal.Proposal.StartEpoch, deal.Proposal.EndEpoch, pieceSize, pieceReader)
}

// LocatePieceForDealWithinSector finds the sector, offset and length of a piece associated with the given deal id
func (s *StorageProviderNodeConnector) LocatePieceForDealWithinSector(ctx context.Context, dealID abi.DealID, tok shared.TipSetToken) (sectorNumber uint64, offset uint64, length uint64, err error) {
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
		k, err := adt.ParseIntKey(key)
		if err != nil {
			return err
		}
		sectorNumber = uint64(k)

		for _, deal := range sectorInfo.Info.DealIDs {
			if deal == dealID {
				offset = uint64(0)
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
						sectorNumber = uint64(k)
						length = uint64(proposal.PieceSize)
						return nil // Found!
					}
					offset += uint64(proposal.PieceSize)
				}
			}
		}
		return errors.New("Deal not found")
	})
	return
}
