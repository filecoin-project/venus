package storagemarketconnector

import (
	"context"

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

type ConnectorCommon struct {
	chainStore chainReader
	waiter     *msg.Waiter
	wallet     *wallet.Wallet
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
