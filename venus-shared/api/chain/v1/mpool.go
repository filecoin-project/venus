package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type IMessagePool interface {
	MpoolDeleteByAdress(ctx context.Context, addr address.Address) error                                                                                               //perm:admin
	MpoolPublishByAddr(context.Context, address.Address) error                                                                                                         //perm:write
	MpoolPublishMessage(ctx context.Context, smsg *types.SignedMessage) error                                                                                          //perm:write
	MpoolPush(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error)                                                                                         //perm:write
	MpoolGetConfig(context.Context) (*types.MpoolConfig, error)                                                                                                        //perm:read
	MpoolSetConfig(ctx context.Context, cfg *types.MpoolConfig) error                                                                                                  //perm:admin
	MpoolSelect(context.Context, types.TipSetKey, float64) ([]*types.SignedMessage, error)                                                                             //perm:read
	MpoolSelects(context.Context, types.TipSetKey, []float64) ([][]*types.SignedMessage, error)                                                                        //perm:read
	MpoolPending(ctx context.Context, tsk types.TipSetKey) ([]*types.SignedMessage, error)                                                                             //perm:read
	MpoolClear(ctx context.Context, local bool) error                                                                                                                  //perm:write
	MpoolPushUntrusted(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error)                                                                                //perm:write
	MpoolPushMessage(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec) (*types.SignedMessage, error)                                               //perm:sign
	MpoolBatchPush(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error)                                                                               //perm:write
	MpoolBatchPushUntrusted(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error)                                                                      //perm:write
	MpoolBatchPushMessage(ctx context.Context, msgs []*types.Message, spec *types.MessageSendSpec) ([]*types.SignedMessage, error)                                     //perm:sign
	MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error)                                                                                           //perm:read
	MpoolSub(ctx context.Context) (<-chan types.MpoolUpdate, error)                                                                                                    //perm:read
	GasEstimateMessageGas(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec, tsk types.TipSetKey) (*types.Message, error)                           //perm:read
	GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*types.EstimateMessage, fromNonce uint64, tsk types.TipSetKey) ([]*types.EstimateResult, error) //perm:read
	GasEstimateFeeCap(ctx context.Context, msg *types.Message, maxqueueblks int64, tsk types.TipSetKey) (big.Int, error)                                               //perm:read
	GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk types.TipSetKey) (big.Int, error)                       //perm:read
	GasEstimateGasLimit(ctx context.Context, msgIn *types.Message, tsk types.TipSetKey) (int64, error)                                                                 //perm:read
	// MpoolCheckMessages performs logical checks on a batch of messages
	MpoolCheckMessages(ctx context.Context, protos []*types.MessagePrototype) ([][]types.MessageCheckStatus, error) //perm:read
	// MpoolCheckPendingMessages performs logical checks for all pending messages from a given address
	MpoolCheckPendingMessages(ctx context.Context, addr address.Address) ([][]types.MessageCheckStatus, error) //perm:read
	// MpoolCheckReplaceMessages performs logical checks on pending messages with replacement
	MpoolCheckReplaceMessages(ctx context.Context, msg []*types.Message) ([][]types.MessageCheckStatus, error) //perm:read
}
