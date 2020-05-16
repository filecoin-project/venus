package storagemarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	spasm "github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

var log = logging.Logger("storage-protocol")

type chainReader interface {
	Head() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetStateRoot(ctx context.Context, tipKey block.TipSetKey) (cid.Cid, error)
	GetActorStateAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address, out interface{}) error
	StateView(key block.TipSetKey) (*state.View, error)
	cbor.IpldStore
}

// WorkerGetter is a function that can retrieve the miner worker for the given address from actor state
type WorkerGetter func(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)

type connectorCommon struct {
	chainStore  chainReader
	stateViewer *appstate.Viewer
	waiter      *msg.Waiter
	signer      types.Signer
	outbox      *message.Outbox
}

// MostRecentStateId returns the state key from the current head of the chain.
func (c *connectorCommon) GetChainHead(_ context.Context) (shared.TipSetToken, abi.ChainEpoch, error) { // nolint: golint
	return connectors.GetChainHead(c.chainStore)
}

func (c *connectorCommon) WaitForMessage(ctx context.Context, mcid cid.Cid, onCompletion func(exitcode.ExitCode, []byte, error) error) error {
	return c.waiter.Wait(ctx, mcid, msg.DefaultMessageWaitLookback, func(b *block.Block, message *types.SignedMessage, r *vm.MessageReceipt) error {
		return onCompletion(r.ExitCode, r.ReturnValue, nil)
	})
}

func (c *connectorCommon) addFunds(ctx context.Context, fromAddr address.Address, addr address.Address, amount abi.TokenAmount) (cid.Cid, error) {
	mcid, _, err := c.outbox.Send(
		ctx,
		fromAddr,
		builtin.StorageMarketActorAddr,
		types.NewAttoFIL(amount.Int),
		types.NewGasPrice(1),
		gas.NewGas(5000),
		true,
		builtin.MethodsMarket.AddBalance,
		&addr,
	)
	return mcid, err
}

// SignBytes uses the local wallet to sign the bytes with the given address
func (c *connectorCommon) SignBytes(ctx context.Context, signer address.Address, b []byte) (*crypto.Signature, error) {
	sig, err := c.signer.SignBytes(ctx, b, signer)
	return &sig, err
}

func (c *connectorCommon) GetBalance(ctx context.Context, addr address.Address, tok shared.TipSetToken) (storagemarket.Balance, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return storagemarket.Balance{}, xerrors.Errorf("failed to marshal TipSetToken into a TipSetKey: %w", err)
	}

	// Direct state access should be replaced with use of the state view.
	var smState spasm.State
	err := c.chainStore.GetActorStateAt(ctx, tsk, builtin.StorageMarketActorAddr, &smState)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	view, err := c.chainStore.StateView(tsk)
	if err != nil {
		return storagemarket.Balance{}, err
	}
	resAddr, err := view.InitResolveAddress(ctx, addr)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	available, err := c.getBalance(ctx, smState.EscrowTable, resAddr)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	locked, err := c.getBalance(ctx, smState.LockedTable, resAddr)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	return storagemarket.Balance{
		Available: abi.NewTokenAmount(available.Int64()),
		Locked:    abi.NewTokenAmount(locked.Int64()),
	}, nil
}

func (c *connectorCommon) GetMinerWorkerAddress(ctx context.Context, miner address.Address, tok shared.TipSetToken) (address.Address, error) {
	view, err := c.loadStateView(tok)
	if err != nil {
		return address.Undef, err
	}

	_, fcworker, err := view.MinerControlAddresses(ctx, miner)
	if err != nil {
		return address.Undef, err
	}

	return fcworker, nil
}

func (c *connectorCommon) OnDealSectorCommitted(ctx context.Context, provider address.Address, dealID abi.DealID, cb storagemarket.DealSectorCommittedCallback) error {
	view, err := c.chainStore.StateView(c.chainStore.Head())
	if err != nil {
		cb(err)
		return err
	}

	resolvedProvider, err := view.InitResolveAddress(ctx, provider)
	if err != nil {
		cb(err)
		return err
	}

	err = c.waiter.WaitPredicate(ctx, msg.DefaultMessageWaitLookback, func(msg *types.SignedMessage, msgCid cid.Cid) bool {
		resolvedTo, err := view.InitResolveAddress(ctx, msg.Message.To)
		if err != nil {
			return false
		}

		if resolvedTo != resolvedProvider {
			return false
		}

		if msg.Message.Method != builtin.MethodsMiner.ProveCommitSector {
			return false
		}

		// that's enough for us to check chain state
		view, err = c.chainStore.StateView(c.chainStore.Head())
		if err != nil {
			return false
		}

		_, found, err := view.MarketDealState(ctx, dealID)
		if err != nil {
			return false
		}

		return found
	}, func(b *block.Block, signedMessage *types.SignedMessage, receipt *vm.MessageReceipt) error {
		return nil
	})

	cb(err)
	return err
}

func (c *connectorCommon) getBalance(ctx context.Context, root cid.Cid, addr address.Address) (abi.TokenAmount, error) {
	// These should be replaced with methods on the state view
	table, err := adt.AsBalanceTable(state.StoreFromCbor(ctx, c.chainStore), root)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	hasBalance, err := table.Has(addr)
	if err != nil {
		return big.Zero(), err
	}
	balance := abi.NewTokenAmount(0)
	if hasBalance {
		balance, err = table.Get(addr)
		if err != nil {
			return big.Zero(), err
		}
	}
	return balance, nil
}

func (c *connectorCommon) listDeals(ctx context.Context, tok shared.TipSetToken, predicate func(proposal *spasm.DealProposal, dealState *spasm.DealState) bool) ([]storagemarket.StorageDeal, error) {
	view, err := c.loadStateView(tok)
	if err != nil {
		return nil, err
	}

	// Deals are not indexed in (expensive) chain state.
	// This iterates *all* deal states, loads the associated proposals, and filters by provider.
	// This is going to be really slow until we find a place to index deals, either here or in the module.
	deals := []storagemarket.StorageDeal{}
	err = view.MarketDealStatesForEach(ctx, func(dealId abi.DealID, state *spasm.DealState) error {
		proposal, err := view.MarketDealProposal(ctx, dealId)
		if err != nil {
			return xerrors.Errorf("no proposal for deal %d: %w", dealId, err)
		}
		if predicate(&proposal, state) {
			deals = append(deals, storagemarket.StorageDeal{
				DealProposal: proposal,
				DealState:    *state,
			})
		}
		return nil
	})
	return deals, err
}

func (c *connectorCommon) VerifySignature(ctx context.Context, signature crypto.Signature, signer address.Address, plaintext []byte, tok shared.TipSetToken) (bool, error) {
	view, err := c.loadStateView(tok)
	if err != nil {
		return false, err
	}

	validator := state.NewSignatureValidator(view)

	return nil == validator.ValidateSignature(ctx, plaintext, signer, signature), nil
}

func (c *connectorCommon) loadStateView(tok shared.TipSetToken) (*appstate.View, error) {
	var tsk block.TipSetKey
	if err := encoding.Decode(tok, &tsk); err != nil {
		return nil, xerrors.Errorf("failed to marshal tok to a tipset key: %w", err)
	}
	return c.chainStore.StateView(tsk)
}
