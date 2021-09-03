package events

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/types"
)

type uncachedAPI interface {
	ChainNotify(context.Context) (<-chan []*chain.HeadChange, error)
	ChainGetPath(ctx context.Context, from, to types.TipSetKey) ([]*chain.HeadChange, error)
	StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*chain.MsgLookup, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) // optional / for CalledMsg
}

type cache struct {
	*tipSetCache
	*messageCache
	uncachedAPI
}

func newCache(api IEvent, gcConfidence abi.ChainEpoch) *cache {
	return &cache{
		newTSCache(api, gcConfidence),
		newMessageCache(api),
		api,
	}
}
