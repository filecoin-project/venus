package storagemarketconnector

import (
	"context"

	"github.com/filecoin-project/go-address"
	smtypes "github.com/filecoin-project/go-fil-markets/shared/types"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

// Implements storagemarket.StateKey
type stateKey struct {
	ts     block.TipSetKey
	height uint64
}

func (k *stateKey) Height() uint64 {
	return k.height
}

type ConnectorCommon struct {
	chainStore *chain.Store
	waiter     *msg.Waiter
	wallet     *wallet.Wallet
}

func (c *ConnectorCommon) MostRecentStateId(ctx context.Context) (storagemarket.StateKey, error) {
	key := c.chainStore.GetHead()
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
	// TODO: how to read from StorageMarketActor state
	panic("TODO: go-fil-markets integration")
}
