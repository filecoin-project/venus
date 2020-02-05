package storagemarketconnector

import (
	"context"

	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"

	"github.com/filecoin-project/go-fil-markets/shared/tokenamount"

	"github.com/filecoin-project/go-address"
	smtypes "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	spaabi "github.com/filecoin-project/specs-actors/actors/abi"
	spasm "github.com/filecoin-project/specs-actors/actors/builtin/storage_market"
	spautil "github.com/filecoin-project/specs-actors/actors/util"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	fcsm "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemarket"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type chainReader interface {
	Head() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetActorStateAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address, out interface{}) error
}

// Implements storagemarket.StateKey
type stateKey struct {
	ts     block.TipSetKey
	height uint64
}

func (k *stateKey) Height() uint64 {
	return k.height
}

// WorkerGetter is a function that can retrieve the miner worker for the given address from actor state
type WorkerGetter func(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)

type connectorCommon struct {
	chainStore   chainReader
	waiter       *msg.Waiter
	wallet       *wallet.Wallet
	outbox       *message.Outbox
	workerGetter WorkerGetter
}

// MostRecentStateId returns the state key from the current head of the chain.
func (c *connectorCommon) MostRecentStateId(ctx context.Context) (storagemarket.StateKey, error) { // nolint: golint
	key := c.chainStore.Head()
	ts, err := c.chainStore.GetTipSet(key)
	if err != nil {
		return nil, err
	}

	height, err := ts.Height()
	if err != nil {
		return nil, err
	}

	return &stateKey{key, height}, nil
}

func (c *connectorCommon) wait(ctx context.Context, mcid cid.Cid, pubErrCh chan error) (*types.MessageReceipt, error) {
	receiptChan := make(chan *types.MessageReceipt)
	errChan := make(chan error)

	err := <-pubErrCh
	if err != nil {
		return nil, err
	}

	go func() {
		err := c.waiter.Wait(ctx, mcid, func(b *block.Block, message *types.SignedMessage, r *types.MessageReceipt) error {
			receiptChan <- r
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case receipt := <-receiptChan:
		if receipt.ExitCode != 0 {
			return nil, xerrors.Errorf("non-zero exit code: %d", receipt.ExitCode)
		}

		return receipt, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, xerrors.New("context ended prematurely")
	}
}

func (c *connectorCommon) addFunds(ctx context.Context, fromAddr address.Address, addr address.Address, amount tokenamount.TokenAmount) error {
	params, err := abi.ToEncodedValues(addr)
	if err != nil {
		return err
	}

	mcid, cerr, err := c.outbox.Send(
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

	_, err = c.wait(ctx, mcid, cerr)

	return err
}

// SignBytes uses the local wallet to sign the bytes with the given address
func (c *connectorCommon) SignBytes(ctx context.Context, signer address.Address, b []byte) (*smtypes.Signature, error) {
	var err error

	fcSig, err := c.wallet.SignBytes(b, signer)
	if err != nil {
		return nil, err
	}

	var sigType string
	if signer.Protocol() == address.BLS {
		sigType = smtypes.KTBLS
	} else {
		sigType = smtypes.KTSecp256k1
	}
	return &smtypes.Signature{
		Type: sigType,
		Data: fcSig[:],
	}, nil
}

func (c *connectorCommon) GetBalance(ctx context.Context, addr address.Address) (storagemarket.Balance, error) {
	var smState spasm.StorageMarketActorState
	err := c.chainStore.GetActorStateAt(ctx, c.chainStore.Head(), fcaddr.StorageMarketAddress, &smState)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	// Dragons: Balance or similar should be an exported method on StorageMarketState. Do it ourselves for now.
	available, ok := spautil.BalanceTable_GetEntry(smState.EscrowTable, addr)
	if !ok {
		available = spaabi.NewTokenAmount(0)
	}

	locked, ok := spautil.BalanceTable_GetEntry(smState.LockedReqTable, addr)
	if !ok {
		locked = spaabi.NewTokenAmount(0)
	}

	return storagemarket.Balance{
		Available: tokenamount.FromInt(available.Int.Uint64()),
		Locked:    tokenamount.FromInt(locked.Int.Uint64()),
	}, nil
}

func (c *connectorCommon) GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error) {
	// Fetch from chain
	fcworker, err := c.workerGetter(ctx, miner, c.chainStore.Head())
	if err != nil {
		return address.Undef, err
	}

	// Convert back to go-address
	return address.NewFromBytes(fcworker.Bytes())
}

func (c *connectorCommon) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID uint64, cb storagemarket.DealSectorCommittedCallback) error {
	// TODO: is this provider address the miner address or the miner worker address?

	pred := func(msg *types.SignedMessage, msgCid cid.Cid) bool {
		m := msg.Message
		if m.Method != fcsm.CommitSector {
			return false
		}

		// TODO: compare addresses directly when they share a type #3719
		if m.From.String() != provider.String() {
			return false
		}

		values, err := abi.DecodeValues(m.Params, []abi.Type{abi.SectorProveCommitInfo})
		if err != nil {
			return false
		}

		commitInfo := values[0].Val.(*types.SectorProveCommitInfo)
		for _, id := range commitInfo.DealIDs {
			if uint64(id) == dealID {
				return true
			}
		}
		return false
	}

	msg, found, err := c.waiter.Find(ctx, pred)
	if err != nil {
		cb(0, err)
		return err
	}
	if found {
		sectorID, err := decodeSectorId(msg.Message)
		cb(sectorID, err)
		return err
	}

	return c.waiter.WaitPredicate(ctx, pred, func(_ *block.Block, msg *types.SignedMessage, _ *types.MessageReceipt) error {
		sectorID, err := decodeSectorId(msg)
		cb(sectorID, err)
		return err
	})
}

func decodeSectorId(msg *types.SignedMessage) (uint64, error) {
	values, err := abi.DecodeValues(msg.Message.Params, []abi.Type{abi.SectorProveCommitInfo})
	if err != nil {
		return 0, err
	}

	commitInfo, ok := values[0].Val.(*types.SectorProveCommitInfo)
	if !ok {
		return 0, errors.Errorf("Expected message params to be SectorProveCommitInfo, but was %v", values[0].Type)
	}

	return uint64(commitInfo.SectorID), nil
}

func (c *connectorCommon) listDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	var smState spasm.StorageMarketActorState
	err := c.chainStore.GetActorStateAt(ctx, c.chainStore.Head(), fcaddr.StorageMarketAddress, &smState)
	if err != nil {
		return nil, err
	}

	// Dragons: ListDeals or similar should be an exported method on StorageMarketState. Do it ourselves for now.
	providerDealIds, ok := smState.CachedDealIDsByParty[addr]
	if !ok {
		return nil, errors.Errorf("No deals for %s", addr.String())
	}

	deals := []storagemarket.StorageDeal{}
	for dealID := range providerDealIds {
		onChainDeal, ok := smState.Deals[dealID]
		if !ok {
			return nil, errors.Errorf("Could not find deal for id %d", dealID)
		}
		proposal := onChainDeal.Deal.Proposal
		deals = append(deals, storagemarket.StorageDeal{
			// Dragons: We're almost certainly looking for a CommP here.
			PieceRef:             proposal.PieceCID.Bytes(),
			PieceSize:            uint64(proposal.PieceSize.Total()),
			Client:               proposal.Client,
			Provider:             proposal.Provider,
			ProposalExpiration:   uint64(proposal.EndEpoch),
			Duration:             uint64(proposal.Duration()),
			StoragePricePerEpoch: tokenamount.FromInt(proposal.StoragePricePerEpoch.Int.Uint64()),
			StorageCollateral:    tokenamount.FromInt(proposal.ProviderCollateral.Int.Uint64()),
			ActivationEpoch:      uint64(proposal.StartEpoch),
		})
	}

	return deals, nil
}
