package localstore

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/ipfs/go-cid"
)

type TipSetLoader interface {
	GetTipSet(context.Context, chain.TipSetKey) (*chain.TipSet, error)
}

type MessageLoader interface {
	ReadMsgMetaCids(ctx context.Context, mmc cid.Cid) ([]cid.Cid, []cid.Cid, error)

	LoadMessagesFromCids(cids []cid.Cid) ([]*chain.Message, error)
	LoadSignedMessagesFromCids(cids []cid.Cid) ([]*chain.SignedMessage, error)
}

type ChainLoader interface {
	TipSetLoader
	MessageLoader
}
