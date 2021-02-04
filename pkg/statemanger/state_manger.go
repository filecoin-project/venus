package statemanger

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/types"
	"golang.org/x/xerrors"
	"time"
)

// stateManagerAPI defines the methods needed from StateManager
type IStateManager interface {
	ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error)
	GetPaychState(ctx context.Context, addr address.Address, ts *block.TipSet) (*types.Actor, paych.State, error)
	Call(ctx context.Context, msg *types.UnsignedMessage, ts *block.TipSet) (*types.InvocResult, error)
	GetMarketState(ctx context.Context, ts *block.TipSet) (market.State, error)
}

type stmgr struct {
	crw cst.IChainReadWriter
	cp  consensus.Protocol
}

func NewStateMangerAPI(crw cst.IChainReadWriter, cp consensus.Protocol) IStateManager {
	return &stmgr{
		crw: crw,
		cp:  cp,
	}
}

func (o *stmgr) ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error) {
	switch addr.Protocol() {
	case address.BLS, address.SECP256K1:
		return addr, nil
	case address.Actor:
		return address.Undef, xerrors.New("cannot resolve actor address to key address")
	default:
	}
	if ts == nil {
		ts = o.crw.Head()
	}
	view, err := o.crw.StateView(ts)
	if err != nil {
		return address.Undef, err
	}
	return view.ResolveToKeyAddr(ctx, addr)
}

func (o *stmgr) Call(ctx context.Context, msg *types.UnsignedMessage, ts *block.TipSet) (*types.InvocResult, error) {
	timeStart := time.Now()
	msgCid, err := msg.Cid()
	if err != nil {
		return nil, err
	}
	ret, err := o.cp.Call(ctx, msg, ts)
	if err != nil {
		return nil, err
	}
	return &types.InvocResult{
		MsgCid:         msgCid,
		Msg:            msg,
		MsgRct:         &ret.Receipt,
		ExecutionTrace: &ret.GasTracker.ExecutionTrace,
		Duration:       time.Now().Sub(timeStart),
	}, nil
}
func (o *stmgr) GetPaychState(ctx context.Context, addr address.Address, ts *block.TipSet) (*types.Actor, paych.State, error) {
	if ts == nil {
		ts = o.crw.Head()
	}
	view, err := o.crw.ParentStateView(ts)
	if err != nil {
		return nil, nil, err
	}
	act, err := view.LoadActor(ctx, addr)
	if err != nil {
		return nil, nil, err
	}
	actState, err := view.LoadPaychState(ctx, act)
	if err != nil {
		return nil, nil, err
	}
	return act, actState, nil
}
func (o *stmgr) GetMarketState(ctx context.Context, ts *block.TipSet) (market.State, error) {
	if ts == nil {
		ts = o.crw.Head()
	}
	view, err := o.crw.ParentStateView(ts)
	if err != nil {
		return nil, err
	}
	actState, err := view.LoadMarketState(ctx)
	if err != nil {
		return nil, err
	}
	return actState, nil
}
