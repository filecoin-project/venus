package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/chain"
	"github.com/filecoin-project/venus/venus-shared/messagepool"
)

type IMessagePool interface {
	// Rule[perm:read]
	MpoolDeleteByAdress(ctx context.Context, addr address.Address) error
	// Rule[perm:read]
	MpoolPublishByAddr(context.Context, address.Address) error
	// Rule[perm:read]
	MpoolPublishMessage(ctx context.Context, smsg *chain.SignedMessage) error
	// Rule[perm:read]
	MpoolPush(ctx context.Context, smsg *chain.SignedMessage) (cid.Cid, error)
	// Rule[perm:read]
	MpoolGetConfig(context.Context) (*messagepool.MpoolConfig, error)
	// Rule[perm:read]
	MpoolSetConfig(ctx context.Context, cfg *messagepool.MpoolConfig) error
	// Rule[perm:read]
	MpoolSelect(context.Context, chain.TipSetKey, float64) ([]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolSelects(context.Context, chain.TipSetKey, []float64) ([][]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolPending(ctx context.Context, tsk chain.TipSetKey) ([]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolClear(ctx context.Context, local bool) error
	// Rule[perm:read]
	MpoolPushUntrusted(ctx context.Context, smsg *chain.SignedMessage) (cid.Cid, error)
	// Rule[perm:read]
	MpoolPushMessage(ctx context.Context, msg *chain.Message, spec *MessageSendSpec) (*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolBatchPush(ctx context.Context, smsgs []*chain.SignedMessage) ([]cid.Cid, error)
	// Rule[perm:read]
	MpoolBatchPushUntrusted(ctx context.Context, smsgs []*chain.SignedMessage) ([]cid.Cid, error)
	// Rule[perm:read]
	MpoolBatchPushMessage(ctx context.Context, msgs []*chain.Message, spec *MessageSendSpec) ([]*chain.SignedMessage, error)
	// Rule[perm:read]
	MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error)
	// Rule[perm:read]
	MpoolSub(ctx context.Context) (<-chan messagepool.MpoolUpdate, error)
	// Rule[perm:read]
	GasEstimateMessageGas(ctx context.Context, msg *chain.Message, spec *MessageSendSpec, tsk chain.TipSetKey) (*chain.Message, error)
	// Rule[perm:read]
	GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*EstimateMessage, fromNonce uint64, tsk chain.TipSetKey) ([]*EstimateResult, error)
	// Rule[perm:read]
	GasEstimateFeeCap(ctx context.Context, msg *chain.Message, maxqueueblks int64, tsk chain.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk chain.TipSetKey) (big.Int, error)
	// Rule[perm:read]
	GasEstimateGasLimit(ctx context.Context, msgIn *chain.Message, tsk chain.TipSetKey) (int64, error)
	// MpoolCheckMessages performs logical checks on a batch of messages
	// Rule[perm:read]
	MpoolCheckMessages(ctx context.Context, protos []*messagepool.MessagePrototype) ([][]messagepool.MessageCheckStatus, error)
	// MpoolCheckPendingMessages performs logical checks for all pending messages from a given address
	// Rule[perm:read]
	MpoolCheckPendingMessages(ctx context.Context, addr address.Address) ([][]messagepool.MessageCheckStatus, error)
	// MpoolCheckReplaceMessages performs logical checks on pending messages with replacement
	// Rule[perm:read]
	MpoolCheckReplaceMessages(ctx context.Context, msg []*chain.Message) ([][]messagepool.MessageCheckStatus, error)
}
