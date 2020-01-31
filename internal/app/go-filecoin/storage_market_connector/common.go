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
	GetActorStateAt(ctx context.Context, tipKey block.TipSetKey, addr fcaddr.Address, out interface{}) error
}

// Implements storagemarket.StateKey
type stateKey struct {
	ts     block.TipSetKey
	height uint64
}

func (k *stateKey) Height() uint64 {
	return k.height
}

type WorkerGetter func(ctx context.Context, minerAddr fcaddr.Address, baseKey block.TipSetKey) (fcaddr.Address, error)

type ConnectorCommon struct {
	chainStore   chainReader
	waiter       *msg.Waiter
	wallet       *wallet.Wallet
	outbox       *message.Outbox
	workerGetter WorkerGetter
}

func (c *ConnectorCommon) MostRecentStateId(ctx context.Context) (storagemarket.StateKey, error) {
	key := c.chainStore.Head()
	ts, err := c.chainStore.GetTipSet(key)

	if err != nil {
		return nil, err
	}

	return &stateKey{key, uint64(ts.At(0).Height)}, nil
}

func (c *ConnectorCommon) wait(ctx context.Context, mcid cid.Cid, pubErrCh chan error) (*types.MessageReceipt, error) {
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

func (c *ConnectorCommon) addFunds(ctx context.Context, from address.Address, addr address.Address, amount tokenamount.TokenAmount) error {
	params, err := abi.ToEncodedValues(addr)
	if err != nil {
		return err
	}

	fromAddr, err := fcaddr.NewFromBytes(from.Bytes())
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

func (s *ConnectorCommon) SignBytes(ctx context.Context, signer address.Address, b []byte) (*smtypes.Signature, error) {
	var fcSigner fcaddr.Address
	var err error

	if fcSigner, err = fcaddr.NewFromBytes(signer.Bytes()); err != nil {
		return nil, err
	}

	fcSig, err := s.wallet.SignBytes(b, fcSigner)
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

func (c *ConnectorCommon) GetBalance(ctx context.Context, addr address.Address) (storagemarket.Balance, error) {
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

func (c *ConnectorCommon) GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error) {
	// Convert to FC address
	fcMiner, err := fcaddr.NewFromBytes(miner.Bytes())
	if err != nil {
		return address.Undef, err
	}

	// Fetch from chain
	fcworker, err := c.workerGetter(ctx, fcMiner, c.chainStore.Head())
	if err != nil {
		return address.Undef, err
	}

	// Convert back to go-address
	return address.NewFromBytes(fcworker.Bytes())
}

func (s *ConnectorCommon) getFCWorker(ctx context.Context, providerAddr address.Address) (fcaddr.Address, error) {
	worker, err := s.GetMinerWorker(ctx, providerAddr)
	if err != nil {
		return fcaddr.Undef, err
	}

	workerAddr, err := fcaddr.NewFromBytes(worker.Bytes())
	if err != nil {
		return fcaddr.Undef, err
	}
	return workerAddr, nil
}

func (s *ConnectorCommon) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID uint64, cb storagemarket.DealSectorCommittedCallback) error {
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

	_, found, err := s.waiter.Find(ctx, pred)
	if found {
		// TODO: DealSectorCommittedCallback should take a sector ID which we would provide here.
		cb(err)
		return nil
	}

	return s.waiter.WaitPredicate(ctx, pred, func(_ *block.Block, _ *types.SignedMessage, _ *types.MessageReceipt) error {
		// TODO: DealSectorCommittedCallback should take a sector ID which we would provide here.
		cb(nil)
		return nil
	})
}

func (c *ConnectorCommon) listDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
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
	for dealId, _ := range providerDealIds {
		onChainDeal, ok := smState.Deals[dealId]
		if !ok {
			return nil, errors.Errorf("Could not find deal for id %d", dealId)
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
