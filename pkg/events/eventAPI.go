package events

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/block"
	chain2 "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

type IEvent interface {
	ChainNotify(context.Context) chan []*chain2.HeadChange
	ChainGetBlockMessages(context.Context, cid.Cid) (*chain.BlockMessages, error)
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	ChainHead(context.Context) (*block.TipSet, error)
	StateGetReceipt(context.Context, cid.Cid, block.TipSetKey) (*types.MessageReceipt, error)
	ChainGetTipSet(block.TipSetKey) (*block.TipSet, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) // optional / for CalledMsg
}

/*type event struct { //notlint
	chain.IChain
}

func newEventAPI(ca chain.IChain) IEvent { //notlint
	return ca
}*/
