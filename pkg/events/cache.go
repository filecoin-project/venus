package events

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type uncachedAPI interface {
	ChainNotify(context.Context) (<-chan []*types.HeadChange, error)
	ChainGetPath(ctx context.Context, from, to types.TipSetKey) ([]*types.HeadChange, error)
	StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) // optional / for CalledMsg

	ChainGetTipSetAfterHeight(context.Context, abi.ChainEpoch, types.TipSetKey) (*types.TipSet, error)
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, types.TipSetKey) (*types.TipSet, error)
	ChainGetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	ChainHead(context.Context) (*types.TipSet, error)
}

type cache struct {
	//*tipSetCache
	*messageCache
	uncachedAPI
}

func newCache(api IEvent, gcConfidence abi.ChainEpoch) *cache {
	return &cache{
		//newTSCache(api, gcConfidence),
		newMessageCache(api),
		api,
	}
}
