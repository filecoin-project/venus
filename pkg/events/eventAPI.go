package events

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

// A TipSetObserver receives notifications of tipsets
type TipSetObserver interface {
	Apply(ctx context.Context, from, to *types.TipSet) error
	Revert(ctx context.Context, from, to *types.TipSet) error
}

type IEvent interface {
	ChainNotify(context.Context) (<-chan []*types.HeadChange, error)
	ChainGetBlockMessages(context.Context, cid.Cid) (*types.BlockMessages, error)
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, types.TipSetKey) (*types.TipSet, error)
	ChainGetTipSetAfterHeight(context.Context, abi.ChainEpoch, types.TipSetKey) (*types.TipSet, error)
	ChainHead(context.Context) (*types.TipSet, error)
	StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*types.MsgLookup, error)
	ChainGetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	ChainGetPath(ctx context.Context, from, to types.TipSetKey) ([]*types.HeadChange, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) // optional / for CalledMsg
}
