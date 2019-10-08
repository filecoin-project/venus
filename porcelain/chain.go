package porcelain

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

type chainHeadPlumbing interface {
	ChainHeadKey() types.TipSetKey
	ChainTipSet(key types.TipSetKey) (types.TipSet, error)
}

// ChainHead gets the current head tipset from plumbing.
func ChainHead(plumbing chainHeadPlumbing) (types.TipSet, error) {
	return plumbing.ChainTipSet(plumbing.ChainHeadKey())
}

type fullBlockPlumbing interface {
	ChainGetBlock(context.Context, cid.Cid) (*types.Block, error)
	ChainGetMessages(context.Context, cid.Cid) ([]*types.SignedMessage, error)
	ChainGetReceipts(context.Context, cid.Cid) ([]*types.MessageReceipt, error)
}

// GetFullBlock returns a full block: header, messages, receipts.
func GetFullBlock(ctx context.Context, plumbing fullBlockPlumbing, id cid.Cid) (*types.FullBlock, error) {
	var out types.FullBlock
	var err error

	out.Header, err = plumbing.ChainGetBlock(ctx, id)
	if err != nil {
		return nil, err
	}

	out.Messages, err = plumbing.ChainGetMessages(ctx, out.Header.Messages)
	if err != nil {
		return nil, err
	}

	out.Receipts, err = plumbing.ChainGetReceipts(ctx, out.Header.MessageReceipts)
	if err != nil {
		return nil, err
	}

	return &out, nil
}
