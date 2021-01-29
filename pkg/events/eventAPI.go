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

type EventAPI interface {
	ChainNotify(context.Context) (<-chan []*chain2.HeadChange, error)
	ChainGetBlockMessages(context.Context, cid.Cid) (*chain.BlockMessages, error)
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, block.TipSetKey) (*block.TipSet, error)
	ChainHead(context.Context) (*block.TipSet, error)
	StateGetReceipt(context.Context, cid.Cid, block.TipSetKey) (*types.MessageReceipt, error)
	ChainGetTipSet(context.Context, block.TipSetKey) (*block.TipSet, error)

	StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) // optional / for CalledMsg
}

type event struct {
	*chain.ChainInfoAPI
	*chain.ChainAPI
}

//nolint
func newEventAPI(cia *chain.ChainInfoAPI, ca *chain.ChainAPI) EventAPI {
	return &event{cia, ca}
}

func (o *event) ChainNotify(ctx context.Context) (<-chan []*chain2.HeadChange, error) {
	return o.ChainInfoAPI.ChainNotify(ctx), nil
}
func (o *event) ChainGetBlockMessages(ctx context.Context, c cid.Cid) (*chain.BlockMessages, error) {
	return o.ChainInfoAPI.ChainGetBlockMessages(ctx, c)
}
func (o *event) ChainGetTipSetByHeight(ctx context.Context, ce abi.ChainEpoch, tsk block.TipSetKey) (*block.TipSet, error) {
	return o.ChainInfoAPI.ChainGetTipSetByHeight(ctx, ce, tsk)
}
func (o *event) ChainHead(ctx context.Context) (*block.TipSet, error) {
	return o.ChainInfoAPI.ChainHead(ctx)
}
func (o *event) StateGetReceipt(ctx context.Context, c cid.Cid, tsk block.TipSetKey) (*types.MessageReceipt, error) {
	return o.ChainInfoAPI.StateGetReceipt(ctx, c, tsk)
}
func (o *event) ChainGetTipSet(ctx context.Context, tsk block.TipSetKey) (*block.TipSet, error) {
	return o.ChainInfoAPI.ChainGetTipSet(tsk)
}
func (o *event) StateGetActor(ctx context.Context, actor address.Address, tsk block.TipSetKey) (*types.Actor, error) {
	return o.ChainAPI.StateGetActor(ctx, actor, tsk)
}
