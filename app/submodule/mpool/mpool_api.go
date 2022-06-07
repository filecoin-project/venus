package mpool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/messagepool"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

var _ v1api.IMessagePool = &MessagePoolAPI{}

//MessagePoolAPI messsage pool api implement
type MessagePoolAPI struct {
	pushLocks *messagepool.MpoolLocker

	mp *MessagePoolSubmodule
}

//MpoolDeleteByAdress delete msg in mpool of addr
func (a *MessagePoolAPI) MpoolDeleteByAdress(ctx context.Context, addr address.Address) error {
	return a.mp.MPool.DeleteByAdress(addr)
}

//MpoolPublish publish message of address
func (a *MessagePoolAPI) MpoolPublishByAddr(ctx context.Context, addr address.Address) error {
	return a.mp.MPool.PublishMsgForWallet(ctx, addr)
}

func (a *MessagePoolAPI) MpoolPublishMessage(ctx context.Context, smsg *types.SignedMessage) error {
	return a.mp.MPool.PublishMsg(ctx, smsg)
}

// MpoolPush pushes a signed message to mempool.
func (a *MessagePoolAPI) MpoolPush(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	return a.mp.MPool.Push(ctx, smsg)
}

// MpoolGetConfig returns (a copy of) the current mpool config
func (a *MessagePoolAPI) MpoolGetConfig(context.Context) (*types.MpoolConfig, error) {
	cfg := a.mp.MPool.GetConfig()
	return &types.MpoolConfig{
		PriorityAddrs:          cfg.PriorityAddrs,
		SizeLimitHigh:          cfg.SizeLimitHigh,
		SizeLimitLow:           cfg.SizeLimitLow,
		ReplaceByFeeRatio:      cfg.ReplaceByFeeRatio,
		PruneCooldown:          cfg.PruneCooldown,
		GasLimitOverestimation: cfg.GasLimitOverestimation,
	}, nil
}

// MpoolSetConfig sets the mpool config to (a copy of) the supplied config
func (a *MessagePoolAPI) MpoolSetConfig(ctx context.Context, cfg *types.MpoolConfig) error {
	return a.mp.MPool.SetConfig(ctx, &messagepool.MpoolConfig{
		PriorityAddrs:          cfg.PriorityAddrs,
		SizeLimitHigh:          cfg.SizeLimitHigh,
		SizeLimitLow:           cfg.SizeLimitLow,
		ReplaceByFeeRatio:      cfg.ReplaceByFeeRatio,
		PruneCooldown:          cfg.PruneCooldown,
		GasLimitOverestimation: cfg.GasLimitOverestimation,
	})
}

// MpoolSelect returns a list of pending messages for inclusion in the next block
func (a *MessagePoolAPI) MpoolSelect(ctx context.Context, tsk types.TipSetKey, ticketQuality float64) ([]*types.SignedMessage, error) {
	ts, err := a.mp.chain.API().ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}

	return a.mp.MPool.SelectMessages(ctx, ts, ticketQuality)
}

//MpoolSelects The batch selection message is used when multiple blocks need to select messages at the same time
func (a *MessagePoolAPI) MpoolSelects(ctx context.Context, tsk types.TipSetKey, ticketQualitys []float64) ([][]*types.SignedMessage, error) {
	ts, err := a.mp.chain.API().ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
	}

	return a.mp.MPool.MultipleSelectMessages(ctx, ts, ticketQualitys)
}

// MpoolPending returns pending mempool messages.
func (a *MessagePoolAPI) MpoolPending(ctx context.Context, tsk types.TipSetKey) ([]*types.SignedMessage, error) {
	var ts *types.TipSet
	var err error
	if tsk.IsEmpty() {
		ts, err = a.mp.chain.API().ChainHead(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
		}
	} else {
		ts, err = a.mp.chain.API().ChainGetTipSet(ctx, tsk)
		if err != nil {
			return nil, fmt.Errorf("loading tipset %s: %w", tsk, err)
		}
	}

	pending, mpts := a.mp.MPool.Pending(ctx)

	haveCids := map[cid.Cid]struct{}{}
	for _, m := range pending {
		haveCids[m.Cid()] = struct{}{}
	}

	mptsH := mpts.Height()
	tsH := ts.Height()
	if ts == nil || mptsH > tsH {
		return pending, nil
	}

	for {
		mptsH = mpts.Height()
		tsH = ts.Height()
		if mptsH == tsH {
			if mpts.Equals(ts) {
				return pending, nil
			}
			// different blocks in tipsets

			have, err := a.mp.MPool.MessagesForBlocks(ctx, ts.Blocks())
			if err != nil {
				return nil, fmt.Errorf("getting messages for base ts: %w", err)
			}

			for _, m := range have {
				haveCids[m.Cid()] = struct{}{}
			}
		}

		msgs, err := a.mp.MPool.MessagesForBlocks(ctx, ts.Blocks())
		if err != nil {
			return nil, fmt.Errorf(": %w", err)
		}

		for _, m := range msgs {
			mc := m.Cid()
			if _, ok := haveCids[mc]; ok {
				continue
			}

			haveCids[mc] = struct{}{}
			pending = append(pending, m)
		}

		mptsH = mpts.Height()
		tsH = ts.Height()
		if mptsH >= tsH {
			return pending, nil
		}

		ts, err = a.mp.chain.API().ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("loading parent tipset: %w", err)
		}
	}
}

// MpoolClear clears pending messages from the mpool
func (a *MessagePoolAPI) MpoolClear(ctx context.Context, local bool) error {
	a.mp.MPool.Clear(ctx, local)
	return nil
}

// MpoolPushUntrusted pushes a signed message to mempool from untrusted sources.
func (a *MessagePoolAPI) MpoolPushUntrusted(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	return a.mp.MPool.PushUntrusted(ctx, smsg)
}

// MpoolPushMessage atomically assigns a nonce, signs, and pushes a message
// to mempool.
// maxFee is only used when GasFeeCap/GasPremium fields aren't specified
//
// When maxFee is set to 0, MpoolPushMessage will guess appropriate fee
// based on current chain conditions
func (a *MessagePoolAPI) MpoolPushMessage(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec) (*types.SignedMessage, error) {
	cp := *msg
	msg = &cp
	inMsg := *msg
	fromA, err := a.mp.chain.API().StateAccountKey(ctx, msg.From, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("getting key address: %w", err)
	}
	{
		done, err := a.pushLocks.TakeLock(ctx, fromA)
		if err != nil {
			return nil, fmt.Errorf("taking lock: %w", err)
		}
		defer done()
	}

	if msg.Nonce != 0 {
		return nil, fmt.Errorf("MpoolPushMessage expects message nonce to be 0, was %d", msg.Nonce)
	}

	msg, err = a.GasEstimateMessageGas(ctx, msg, spec, types.TipSetKey{})
	if err != nil {
		return nil, fmt.Errorf("GasEstimateMessageGas error: %w", err)
	}

	if msg.GasPremium.GreaterThan(msg.GasFeeCap) {
		inJSON, err := json.Marshal(inMsg)
		if err != nil {
			return nil, err
		}
		outJSON, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("after estimation, GasPremium is greater than GasFeeCap, inmsg: %s, outmsg: %s",
			inJSON, outJSON)
	}

	if msg.From.Protocol() == address.ID {
		log.Warnf("Push from ID address (%s), adjusting to %s", msg.From, fromA)
		msg.From = fromA
	}

	b, err := a.mp.walletAPI.WalletBalance(ctx, msg.From)
	if err != nil {
		return nil, fmt.Errorf("mpool push: getting origin balance: %w", err)
	}

	if b.LessThan(msg.Value) {
		return nil, fmt.Errorf("mpool push: not enough funds: %s < %s", b, msg.Value)
	}

	// Sign and push the message
	return a.mp.msgSigner.SignMessage(ctx, msg, func(smsg *types.SignedMessage) error {
		if _, err := a.MpoolPush(ctx, smsg); err != nil {
			return fmt.Errorf("mpool push: failed to push message: %w", err)
		}
		return nil
	})
}

// MpoolBatchPush batch pushes a unsigned message to mempool.
func (a *MessagePoolAPI) MpoolBatchPush(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error) {
	var messageCids []cid.Cid
	for _, smsg := range smsgs {
		smsgCid, err := a.mp.MPool.Push(ctx, smsg)
		if err != nil {
			return messageCids, err
		}
		messageCids = append(messageCids, smsgCid)
	}
	return messageCids, nil
}

// MpoolBatchPushUntrusted batch pushes a signed message to mempool from untrusted sources.
func (a *MessagePoolAPI) MpoolBatchPushUntrusted(ctx context.Context, smsgs []*types.SignedMessage) ([]cid.Cid, error) {
	var messageCids []cid.Cid
	for _, smsg := range smsgs {
		smsgCid, err := a.mp.MPool.PushUntrusted(ctx, smsg)
		if err != nil {
			return messageCids, err
		}
		messageCids = append(messageCids, smsgCid)
	}
	return messageCids, nil
}

// MpoolBatchPushMessage batch pushes a unsigned message to mempool.
func (a *MessagePoolAPI) MpoolBatchPushMessage(ctx context.Context, msgs []*types.Message, spec *types.MessageSendSpec) ([]*types.SignedMessage, error) {
	var smsgs []*types.SignedMessage
	for _, msg := range msgs {
		smsg, err := a.MpoolPushMessage(ctx, msg, spec)
		if err != nil {
			return smsgs, err
		}
		smsgs = append(smsgs, smsg)
	}
	return smsgs, nil
}

// MpoolGetNonce gets next nonce for the specified sender.
// Note that this method may not be atomic. Use MpoolPushMessage instead.
func (a *MessagePoolAPI) MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error) {
	return a.mp.MPool.GetNonce(ctx, addr, types.EmptyTSK)
}

func (a *MessagePoolAPI) MpoolSub(ctx context.Context) (<-chan types.MpoolUpdate, error) {
	return a.mp.MPool.Updates(ctx)
}

// GasEstimateMessageGas estimates gas values for unset message gas fields
func (a *MessagePoolAPI) GasEstimateMessageGas(ctx context.Context, msg *types.Message, spec *types.MessageSendSpec, tsk types.TipSetKey) (*types.Message, error) {
	return a.mp.MPool.GasEstimateMessageGas(ctx, &types.EstimateMessage{Msg: msg, Spec: spec}, tsk)
}

func (a *MessagePoolAPI) GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*types.EstimateMessage, fromNonce uint64, tsk types.TipSetKey) ([]*types.EstimateResult, error) {
	return a.mp.MPool.GasBatchEstimateMessageGas(ctx, estimateMessages, fromNonce, tsk)
}

// GasEstimateFeeCap estimates gas fee cap
func (a *MessagePoolAPI) GasEstimateFeeCap(ctx context.Context, msg *types.Message, maxqueueblks int64, tsk types.TipSetKey) (big.Int, error) {
	return a.mp.MPool.GasEstimateFeeCap(ctx, msg, maxqueueblks, tsk)
}

func (a *MessagePoolAPI) GasEstimateGasLimit(ctx context.Context, msgIn *types.Message, tsk types.TipSetKey) (int64, error) {
	return a.mp.MPool.GasEstimateGasLimit(ctx, msgIn, tsk)
}

// GasEstimateGasPremium estimates what gas price should be used for a
// message to have high likelihood of inclusion in `nblocksincl` epochs.
func (a *MessagePoolAPI) GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk types.TipSetKey) (big.Int, error) {
	return a.mp.MPool.GasEstimateGasPremium(ctx, nblocksincl, sender, gaslimit, tsk, a.mp.MPool.PriceCache)
}

func (a *MessagePoolAPI) MpoolCheckMessages(ctx context.Context, protos []*types.MessagePrototype) ([][]types.MessageCheckStatus, error) {
	return a.mp.MPool.CheckMessages(ctx, protos)
}

func (a *MessagePoolAPI) MpoolCheckPendingMessages(ctx context.Context, addr address.Address) ([][]types.MessageCheckStatus, error) {
	return a.mp.MPool.CheckPendingMessages(ctx, addr)
}

func (a *MessagePoolAPI) MpoolCheckReplaceMessages(ctx context.Context, msg []*types.Message) ([][]types.MessageCheckStatus, error) {
	return a.mp.MPool.CheckReplaceMessages(ctx, msg)
}

/*// WalletSign signs the given bytes using the given address.
func (a *MessagePoolAPI) WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error) {
	head := a.mp.chain.ChainReader.GetHead()
	view, err := a.mp.chain.ChainReader.StateView(head)
	if err != nil {
		return nil, err
	}

	keyAddr, err := view.ResolveToKeyAddr(ctx, k)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ID address: %v", keyAddr)
	}
	//var meta wallet.MsgMeta
	//if len(metas) > 0 {
	//	meta = metas[0]
	//} else {
	meta := wallet.MsgMeta{
		Type: wallet.MTUnknown,
	}
	//}
	return a.mp.walletAPI.WalletSign(ctx, keyAddr, msg, meta)
}
*/
