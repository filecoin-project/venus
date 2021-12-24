package market

import (
	"context"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	types "github.com/filecoin-project/venus/venus-shared/chain"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/ipfs/go-cid"
)

// fundManagerAPI is the specific methods called by the FundManager
// (used by the tests)
type fundManager interface {
	MpoolPushMessage(context.Context, *types.Message, *apitypes.MessageSendSpec) (*types.SignedMessage, error)
	StateMarketBalance(context.Context, address.Address, types.TipSetKey) (apitypes.MarketBalance, error)
	StateWaitMsg(ctx context.Context, c cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error)
}

type fmgr struct {
	MPoolAPI      v1api.IMessagePool
	ChainInfoAPI  v1api.IChainInfo
	MinerStateAPI v1api.IMinerState
}

func newFundmanager(p *FundManagerParams) fundManager {
	fmAPI := &fmgr{
		MPoolAPI:      p.MP,
		ChainInfoAPI:  p.CI,
		MinerStateAPI: p.MS,
	}

	return fmAPI
}

func (o *fmgr) MpoolPushMessage(ctx context.Context, msg *types.Message, spec *apitypes.MessageSendSpec) (*types.SignedMessage, error) {
	return o.MPoolAPI.MpoolPushMessage(ctx, msg, spec)
}

func (o *fmgr) StateMarketBalance(ctx context.Context, address address.Address, tsk types.TipSetKey) (apitypes.MarketBalance, error) {
	return o.MinerStateAPI.StateMarketBalance(ctx, address, tsk)
}

func (o *fmgr) StateWaitMsg(ctx context.Context, c cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	return o.ChainInfoAPI.StateWaitMsg(ctx, c, confidence, limit, allowReplaced)
}
