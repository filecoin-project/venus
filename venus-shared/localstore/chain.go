package localstore

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

type TipSetLoader interface {
	GetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
}

type MessageLoader interface {
	ReadMsgMetaCids(ctx context.Context, mmc cid.Cid) ([]cid.Cid, []cid.Cid, error)

	LoadMessagesFromCids(ctx context.Context, cids []cid.Cid) ([]*types.Message, error)
	LoadSignedMessagesFromCids(ctx context.Context, cids []cid.Cid) ([]*types.SignedMessage, error)
}

type ChainLoader interface {
	TipSetLoader
	MessageLoader
}

type FullTipSetLoader interface {
	LoadFullTipSet(ctx context.Context, tsk types.TipSetKey) (*types.FullTipSet, error)
}

type FullTipSetStorer interface {
	StoreFullTipSet(ctx context.Context, fb *types.FullTipSet) error
}
