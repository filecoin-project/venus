package v0api

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"
	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/messagepool"
)

type IMessagePool interface {
	// Rule[perm:admin]
	MpoolDeleteByAdress(ctx context.Context, addr address.Address) error
	// Rule[perm:admin]
	MpoolPublishByAddr(context.Context, address.Address) error
	// Rule[perm:admin]
	MpoolPublishMessage(ctx context.Context, smsg *chain.SignedMessage) error
	// Rule[perm:write]
	MpoolPush(ctx context.Context, smsg *chain.SignedMessage) (cid.Cid, error)
	// Rule[perm:read]
	MpoolGetConfig(context.Context) (*messagepool.MpoolConfig, error)
	// Rule[perm:admin]
	MpoolSetConfig(ctx context.Context, cfg *messagepool.MpoolConfig) error
	// Rule[perm:read]
	MpoolSelect(context.Context, chain.TipSetKey, float64) ([]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolSelects(context.Context, chain.TipSetKey, []float64) ([][]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolPending(ctx context.Context, tsk chain.TipSetKey) ([]*chain.SignedMessage, error)
	// Rule[perm:write]
	MpoolClear(ctx context.Context, local bool) error
	// Rule[perm:write]
	MpoolPushUntrusted(ctx context.Context, smsg *chain.SignedMessage) (cid.Cid, error)
	// Rule[perm:sign]
	MpoolPushMessage(ctx context.Context, msg *chain.Message, spec *chain2.MessageSendSpec) (*chain.SignedMessage, error)
	// Rule[perm:write]
	MpoolBatchPush(ctx context.Context, smsgs []*chain.SignedMessage) ([]cid.Cid, error)
	// Rule[perm:write]
	MpoolBatchPushUntrusted(ctx context.Context, smsgs []*chain.SignedMessage) ([]cid.Cid, error)
	// Rule[perm:sign]
	MpoolBatchPushMessage(ctx context.Context, msgs []*chain.Message, spec *chain2.MessageSendSpec) ([]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error)
	// Rule[perm:read]
	MpoolSub(ctx context.Context) (<-chan messagepool.MpoolUpdate, error)
	// Rule[perm:read]
	GasEstimateMessageGas(ctx context.Context, msg *chain.Message, spec *chain2.MessageSendSpec, tsk chain.TipSetKey) (*chain.Message, error)
	// Rule[perm:read]
	GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*chain2.EstimateMessage, fromNonce uint64, tsk chain.TipSetKey) ([]*chain2.EstimateResult, error)
	// Rule[perm:read]
	GasEstimateFeeCap(ctx context.Context, msg *chain.Message, maxqueueblks int64, tsk chain.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk chain.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	GasEstimateGasLimit(ctx context.Context, msgIn *chain.Message, tsk chain.TipSetKey) (int64, error)
}
