package events

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/chain"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

type IEvent interface {
	ChainNotify(context.Context) chan []*chain2.HeadChange
	ChainGetBlockMessages(context.Context, cid.Cid) (*chain.BlockMessages, error)
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, types.TipSetKey) (*types.TipSet, error)
	ChainHead(context.Context) (*types.TipSet, error)
	StateGetReceipt(context.Context, cid.Cid, types.TipSetKey) (*types.MessageReceipt, error)
	ChainGetTipSet(types.TipSetKey) (*types.TipSet, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk types.TipSetKey) (*types.Actor, error) // optional / for CalledMsg
}

/*type event struct { //notlint
	chain.IChain
}

func newEventAPI(ca chain.IChain) IEvent { //notlint
	return ca
}*/
