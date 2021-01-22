package paychmgr

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/types"
	"time"
)

// stateManagerAPI defines the methods needed from StateManager
type stateManagerAPI interface {
	ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error)
	GetPaychState(ctx context.Context, addr address.Address, ts *block.TipSet) (*types.Actor, paych.State, error)
	Call(ctx context.Context, msg *types.UnsignedMessage, ts *block.TipSet) (*types.InvocResult, error)
}


type stmgr struct {
	cState *cst.ChainStateReadWriter
	cnsns        consensus.Protocol
}

func newStateMangerAPI(cState *cst.ChainStateReadWriter,cnsns   consensus.Protocol) stateManagerAPI {
	return &stmgr{
		cState:cState,
		cnsns:cnsns,
	}
}

func (o *stmgr) ResolveToKeyAddress(ctx context.Context, addr address.Address, ts *block.TipSet) (address.Address, error) {
	return o.cState.ResolveAddressAt(ctx, ts, addr)
}

func (o *stmgr)Call(ctx context.Context, msg *types.UnsignedMessage, ts *block.TipSet) (*types.InvocResult, error){
	timeStart:=time.Now()
	msgCid ,err := msg.Cid()
	if err!=nil{
		return nil, err
	}
	ret,err :=o.cnsns.Call(ctx,msg,ts)
	if err!=nil{
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
func (o *stmgr)GetPaychState(ctx context.Context, addr address.Address, ts *block.TipSet) (*types.Actor, paych.State, error){
	if ts==nil{
		return nil,nil,errors.New("tipset if nil")
	}
	act,err := o.cState.GetActorAt(ctx,ts,addr)
	if err != nil {
		return nil, nil, err
	}
	view,err:= o.cState.ParentStateView(ts)
	if err != nil {
		return nil, nil, err
	}
	actState,err := view.LoadPaychState(ctx,addr)
	if err != nil {
		return nil, nil, err
	}

	return act, actState, nil
}

