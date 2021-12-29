package v0

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
	MpoolDeleteByAdress(ctx context.Context, addr address.Address) error                                                                                                 //perm:admin
	MpoolPublishByAddr(context.Context, address.Address) error                                                                                                           //perm:admin
	MpoolPublishMessage(ctx context.Context, smsg *chain.SignedMessage) error                                                                                            //perm:admin
	MpoolPush(ctx context.Context, smsg *chain.SignedMessage) (cid.Cid, error)                                                                                           //perm:write
	MpoolGetConfig(context.Context) (*messagepool.MpoolConfig, error)                                                                                                    //perm:read
	MpoolSetConfig(ctx context.Context, cfg *messagepool.MpoolConfig) error                                                                                              //perm:admin
	MpoolSelect(context.Context, chain.TipSetKey, float64) ([]*chain.SignedMessage, error)                                                                               //perm:read
	MpoolSelects(context.Context, chain.TipSetKey, []float64) ([][]*chain.SignedMessage, error)                                                                          //perm:read
	MpoolPending(ctx context.Context, tsk chain.TipSetKey) ([]*chain.SignedMessage, error)                                                                               //perm:read
	MpoolClear(ctx context.Context, local bool) error                                                                                                                    //perm:write
	MpoolPushUntrusted(ctx context.Context, smsg *chain.SignedMessage) (cid.Cid, error)                                                                                  //perm:write
	MpoolPushMessage(ctx context.Context, msg *chain.Message, spec *chain2.MessageSendSpec) (*chain.SignedMessage, error)                                                //perm:sign
	MpoolBatchPush(ctx context.Context, smsgs []*chain.SignedMessage) ([]cid.Cid, error)                                                                                 //perm:write
	MpoolBatchPushUntrusted(ctx context.Context, smsgs []*chain.SignedMessage) ([]cid.Cid, error)                                                                        //perm:write
	MpoolBatchPushMessage(ctx context.Context, msgs []*chain.Message, spec *chain2.MessageSendSpec) ([]*chain.SignedMessage, error)                                      //perm:sign
	MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error)                                                                                             //perm:read
	MpoolSub(ctx context.Context) (<-chan messagepool.MpoolUpdate, error)                                                                                                //perm:read
	GasEstimateMessageGas(ctx context.Context, msg *chain.Message, spec *chain2.MessageSendSpec, tsk chain.TipSetKey) (*chain.Message, error)                            //perm:read
	GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*chain2.EstimateMessage, fromNonce uint64, tsk chain.TipSetKey) ([]*chain2.EstimateResult, error) //perm:read
	GasEstimateFeeCap(ctx context.Context, msg *chain.Message, maxqueueblks int64, tsk chain.TipSetKey) (big.Int, error)                                                 //perm:read
	GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk chain.TipSetKey) (big.Int, error)                         //perm:read
	GasEstimateGasLimit(ctx context.Context, msgIn *chain.Message, tsk chain.TipSetKey) (int64, error)                                                                   //perm:read
}
