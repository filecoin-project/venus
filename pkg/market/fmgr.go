package market

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	"github.com/filecoin-project/venus/pkg/block"
)
// fundManagerAPI is the specific methods called by the FundManager
// (used by the tests)
type fundManagerAPI interface {
	MpoolPushMessage(context.Context, *types.UnsignedMessage, *types.MessageSendSpec) (*types.SignedMessage, error)
	StateMarketBalance(context.Context, address.Address, block.TipSetKey) (chain.MarketBalance, error)
	StateWaitMsg(ctx context.Context, c cid.Cid, confidence abi.ChainEpoch) (*cst.MsgLookup, error)
}

type fmgr struct {
	*mpool.MessagePoolAPI
	*chain.ChainInfoAPI
	*chain.MinerStateAPI
}

func NewFundmanagerAPI(p *FundManagerParams)fundManagerAPI{
	return &fmgr{
		p.MessagePoolAPI,
		p.ChainInfoAPI,
		p.MinerStateAPI,
	}
}

func (o *fmgr)MpoolPushMessage(ctx context.Context,msg *types.UnsignedMessage,spec *types.MessageSendSpec) (*types.SignedMessage, error){
	return o.MessagePoolAPI.MpoolPushMessage(ctx,msg,spec)
}

func (o *fmgr)StateMarketBalance(ctx context.Context,address address.Address,tsk block.TipSetKey) (chain.MarketBalance, error){
	return o.MinerStateAPI.StateMarketBalance(ctx,address,tsk)
}

func (o *fmgr)StateWaitMsg(ctx context.Context, c cid.Cid, confidence abi.ChainEpoch) (*cst.MsgLookup, error){
	return o.ChainInfoAPI.StateWaitMsg(ctx,c,confidence)
}