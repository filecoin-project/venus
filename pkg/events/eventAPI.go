package events

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

type IEvent interface {
	ChainNotify(context.Context) (<-chan []*chain2.HeadChange, error)
	ChainGetBlockMessages(context.Context, cid.Cid) (*apitypes.BlockMessages, error)
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, types.TipSetKey) (*types.TipSet, error)
	ChainHead(context.Context) (*types.TipSet, error)
	StateSearchMsg(ctx context.Context, from types.TipSetKey, msg cid.Cid, limit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error)
	ChainGetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) // optional / for CalledMsg
}

/*type event struct { //notlint
	chain.IChain
}

func newEventAPI(ca chain.IChain) IEvent { //notlint
	return ca
}*/
