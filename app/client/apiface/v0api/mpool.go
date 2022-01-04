package v0api

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	types "github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/messagepool"
	"github.com/ipfs/go-cid"
)

type IMessagePool interface {
	// Rule[perm:admin]
	MpoolDeleteByAdress(ctx context.Context, addr address.Address) error
	// Rule[perm:write]
	MpoolPublishByAddr(context.Context, address.Address) error
	// Rule[perm:write]
	MpoolPublishMessage(ctx context.Context, smsg *types.SignedMessage) error
	// Rule[perm:write]
	MpoolPush(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error)
	// Rule[perm:read]
	MpoolGetConfig(context.Context) (*messagepool.MpoolConfig, error)
	// Rule[perm:admin]
	MpoolSetConfig(ctx context.Context, cfg *messagepool.MpoolConfig) error
	// Rule[perm:read]
	MpoolSelect(context.Context, types.TipSetKey, float64) ([]*types.SignedMessage, error)
	// Rule[perm:read]
	MpoolSelects(context.Context, types.TipSetKey, []float64) ([][]*types.SignedMessage, error)
	// Rule[perm:read]
	MpoolPending(ctx context.Context, tsk types.TipSetKey) ([]*types.SignedMessage, error)
	// Rule[perm:write]
	MpoolClear(ctx context.Context, local bool) error
	// Rule[perm:write]
	MpoolPushUntrusted(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error)
	// Rule[perm:sign]
	MpoolPushMessage(ctx context.Context, msg *types.Message, spec *apitypes.MessageSendSpec) (*types.SignedMessage, error)
	// Rule[perm:write]
	MpoolBatchPush(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error)
	// Rule[perm:write]
	MpoolBatchPushUntrusted(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error)
	// Rule[perm:sign]
	MpoolBatchPushMessage(ctx context.Context, msgs []*types.Message, spec *apitypes.MessageSendSpec) ([]*types.SignedMessage, error)
	// Rule[perm:read]
	MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error)
	// Rule[perm:read]
	MpoolSub(ctx context.Context) (<-chan messagepool.MpoolUpdate, error)
	// Rule[perm:read]
	GasEstimateMessageGas(ctx context.Context, msg *types.Message, spec *apitypes.MessageSendSpec, tsk types.TipSetKey) (*types.Message, error)
	// Rule[perm:read]
	GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*apitypes.EstimateMessage, fromNonce uint64, tsk types.TipSetKey) ([]*apitypes.EstimateResult, error)
	// Rule[perm:read]
	GasEstimateFeeCap(ctx context.Context, msg *types.Message, maxqueueblks int64, tsk types.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk types.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	GasEstimateGasLimit(ctx context.Context, msgIn *types.Message, tsk types.TipSetKey) (int64, error)
}