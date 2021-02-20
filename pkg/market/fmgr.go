package market

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/app/submodule/chain"
)

// fundManagerAPI is the specific methods called by the FundManager
// (used by the tests)
type fundManager interface {
	MpoolPushMessage(context.Context, *types.UnsignedMessage, *types.MessageSendSpec) (*types.SignedMessage, error)
	StateMarketBalance(context.Context, address.Address, types.TipSetKey) (chain.MarketBalance, error)
	StateWaitMsg(ctx context.Context, c cid.Cid, confidence abi.ChainEpoch) (*chain.MsgLookup, error)
}

type fmgr struct {
	MPoolAPI      mpool.IMessagePool
	ChainInfoAPI  chain.IChainInfo
	MinerStateAPI chain.IMinerState
}

func newFundmanager(p *FundManagerParams) fundManager {
	fmAPI := &fmgr{
		MPoolAPI:      p.MP,
		ChainInfoAPI:  p.CI,
		MinerStateAPI: p.MS,
	}

	return fmAPI
}

func (o *fmgr) MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec) (*types.SignedMessage, error) {
	return o.MPoolAPI.MpoolPushMessage(ctx, msg, spec)
}

func (o *fmgr) StateMarketBalance(ctx context.Context, address address.Address, tsk types.TipSetKey) (chain.MarketBalance, error) {
	return o.MinerStateAPI.StateMarketBalance(ctx, address, tsk)
}

func (o *fmgr) StateWaitMsg(ctx context.Context, c cid.Cid, confidence abi.ChainEpoch) (*chain.MsgLookup, error) {
	return o.ChainInfoAPI.StateWaitMsg(ctx, c, confidence)
}
