package storagemarketconnector

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors"

	"github.com/filecoin-project/go-fil-markets/shared"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	spasm "github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

type chainReader interface {
	Head() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetStateRoot(ctx context.Context, tipKey block.TipSetKey) (cid.Cid, error)
	GetActorStateAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address, out interface{}) error
	StateView(key block.TipSetKey) (*state.View, error)
	cbor.IpldStore
}

// Implements storagemarket.StateKey
type stateKey struct {
	ts     block.TipSetKey
	height abi.ChainEpoch
}

func (k *stateKey) Height() abi.ChainEpoch {
	return k.height
}

// WorkerGetter is a function that can retrieve the miner worker for the given address from actor state
type WorkerGetter func(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)

type connectorCommon struct {
	chainStore chainReader
	waiter     *msg.Waiter
	wallet     *wallet.Wallet
	outbox     *message.Outbox
}

// MostRecentStateId returns the state key from the current head of the chain.
func (c *connectorCommon) GetChainHead(_ context.Context) (shared.TipSetToken, abi.ChainEpoch, error) { // nolint: golint
	return connectors.GetChainHead(c.chainStore)
}

func (c *connectorCommon) wait(ctx context.Context, mcid cid.Cid, pubErrCh chan error) (*vm.MessageReceipt, error) {
	receiptChan := make(chan *vm.MessageReceipt)
	errChan := make(chan error)

	err := <-pubErrCh
	if err != nil {
		return nil, err
	}

	go func() {
		err := c.waiter.Wait(ctx, mcid, func(b *block.Block, message *types.SignedMessage, r *vm.MessageReceipt) error {
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

func (c *connectorCommon) addFunds(ctx context.Context, fromAddr address.Address, addr address.Address, amount abi.TokenAmount) error {
	mcid, cerr, err := c.outbox.Send(
		ctx,
		fromAddr,
		builtin.StorageMarketActorAddr,
		types.NewAttoFIL(amount.Int),
		types.NewGasPrice(1),
		types.GasUnits(300),
		true,
		builtin.MethodsMarket.AddBalance,
		&addr,
	)
	if err != nil {
		return err
	}

	_, err = c.wait(ctx, mcid, cerr)

	return err
}

// SignBytes uses the local wallet to sign the bytes with the given address
func (c *connectorCommon) SignBytes(ctx context.Context, signer address.Address, b []byte) (*crypto.Signature, error) {
	view, err := c.chainStore.StateView(c.chainStore.Head())
	if err != nil {
		return nil, err
	}
	signer, err = view.AccountSignerAddress(ctx, signer)
	if err != nil {
		return nil, err
	}

	sig, err := c.wallet.SignBytes(b, signer)
	return &sig, err
}

func (c *connectorCommon) GetBalance(ctx context.Context, addr address.Address) (storagemarket.Balance, error) {
	var smState spasm.State
	err := c.chainStore.GetActorStateAt(ctx, c.chainStore.Head(), builtin.StorageMarketActorAddr, &smState)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	available, err := c.getBalance(ctx, smState.EscrowTable, addr)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	locked, err := c.getBalance(ctx, smState.LockedTable, addr)
	if err != nil {
		return storagemarket.Balance{}, err
	}

	return storagemarket.Balance{
		Available: abi.NewTokenAmount(available.Int64()),
		Locked:    abi.NewTokenAmount(locked.Int64()),
	}, nil
}

func (c *connectorCommon) GetMinerWorker(ctx context.Context, miner address.Address) (address.Address, error) {
	// Fetch from chain.
	// TODO: this signature should take a tipset key or state root instead of implicitly using head here
	view, err := c.chainStore.StateView(c.chainStore.Head())
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
	// TODO: is this provider address the miner address or the miner worker address?

	pred := func(msg *types.SignedMessage, msgCid cid.Cid) bool {
		m := msg.Message
		if m.Method != builtin.MethodsMiner.PreCommitSector {
			return false
		}

		if m.From != provider {
			return false
		}

		var params miner.SectorPreCommitInfo
		err := encoding.Decode(m.Params, &params)
		if err != nil {
			return false
		}

		for _, id := range params.DealIDs {
			if id == dealID {
				return true
			}
		}
		return false
	}

	_, found, err := c.waiter.Find(ctx, pred)
	if err != nil {
		cb(err)
		return err
	}
	if found {
		cb(err)
		return err
	}

	return c.waiter.WaitPredicate(ctx, pred, func(_ *block.Block, msg *types.SignedMessage, _ *vm.MessageReceipt) error {
		cb(err)
		return err
	})
}

func (c *connectorCommon) getBalance(ctx context.Context, root cid.Cid, addr address.Address) (abi.TokenAmount, error) {
	// These should be replaced with methods on the state view
	table := adt.AsBalanceTable(state.StoreFromCbor(ctx, c.chainStore), root)
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

func (c *connectorCommon) listDeals(ctx context.Context, addr address.Address) ([]storagemarket.StorageDeal, error) {
	var smState spasm.State
	err := c.chainStore.GetActorStateAt(ctx, c.chainStore.Head(), builtin.StorageMarketActorAddr, &smState)
	if err != nil {
		return nil, err
	}

	// These should be replaced with methods on the state view
	stateStore := state.StoreFromCbor(ctx, c.chainStore)
	byParty := spasm.AsSetMultimap(stateStore, smState.DealIDsByParty)
	var providerDealIds []abi.DealID
	if err = byParty.ForEach(addr, func(i abi.DealID) error {
		providerDealIds = append(providerDealIds, i)
		return nil
	}); err != nil {
		return nil, err
	}

	proposals := adt.AsArray(stateStore, smState.Proposals)
	dealStates := adt.AsArray(stateStore, smState.States)

	deals := []storagemarket.StorageDeal{}
	for _, dealID := range providerDealIds {
		var proposal spasm.DealProposal
		found, err := proposals.Get(uint64(dealID), &proposal)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, errors.Errorf("Could not find deal proposal for id %d", dealID)
		}

		var ds spasm.DealState
		found, err = dealStates.Get(uint64(dealID), &ds)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, errors.Errorf("Could not find deal state for id %d", dealID)
		}

		deals = append(deals, storagemarket.StorageDeal{
			DealProposal: proposal,
			DealState:    ds,
		})
	}

	return deals, nil
}

func (c *connectorCommon) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) bool {
	// This method signature must match that required by the storage market connector, which should be changed
	// to include a context and return error.
	head := c.chainStore.Head()
	view, err := c.chainStore.StateView(head)
	if err != nil {
		return false
	}
	validator := state.NewSignatureValidator(view)
	err = validator.ValidateSignature(context.TODO(), plaintext, signer, signature)
	return err == nil
}
