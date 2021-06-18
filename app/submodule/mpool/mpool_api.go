package mpool

import (
	"context"
	"encoding/json"

	"github.com/filecoin-project/venus/app/submodule/apitypes"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/types"
)

var _ apiface.IMessagePool = &MessagePoolAPI{}

type MessagePoolAPI struct {
	pushLocks *messagepool.MpoolLocker

	mp *MessagePoolSubmodule
}

func (a *MessagePoolAPI) DeleteByAdress(ctx context.Context, addr address.Address) error {
	return a.mp.MPool.DeleteByAdress(addr)
}

func (a *MessagePoolAPI) MpoolPublishByAddr(ctx context.Context, addr address.Address) error {
	return a.mp.MPool.PublishMsgForWallet(ctx, addr)
}

func (a *MessagePoolAPI) MpoolPublishMessage(ctx context.Context, smsg *types.SignedMessage) error {
	return a.mp.MPool.PublishMsg(smsg)
}

func (a *MessagePoolAPI) MpoolPush(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	return a.mp.MPool.Push(ctx, smsg)
}

func (a *MessagePoolAPI) MpoolGetConfig(context.Context) (*messagepool.MpoolConfig, error) {
	return a.mp.MPool.GetConfig(), nil
}

func (a *MessagePoolAPI) MpoolSetConfig(ctx context.Context, cfg *messagepool.MpoolConfig) error {
	return a.mp.MPool.SetConfig(cfg)
}

func (a *MessagePoolAPI) MpoolSelect(ctx context.Context, tsk types.TipSetKey, ticketQuality float64) ([]*types.SignedMessage, error) {
	ts, err := a.mp.chain.API().ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	return a.mp.MPool.SelectMessages(ctx, ts, ticketQuality)
}

func (a *MessagePoolAPI) MpoolSelects(ctx context.Context, tsk types.TipSetKey, ticketQualitys []float64) ([][]*types.SignedMessage, error) {
	ts, err := a.mp.chain.API().ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	return a.mp.MPool.MultipleSelectMessages(ctx, ts, ticketQualitys)
}

func (a *MessagePoolAPI) MpoolPending(ctx context.Context, tsk types.TipSetKey) ([]*types.SignedMessage, error) {
	var ts *types.TipSet
	var err error
	if tsk.IsEmpty() {
		ts, err = a.mp.chain.API().ChainHead(ctx)
		if err != nil {
			return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
		}
	} else {
		ts, err = a.mp.chain.API().ChainGetTipSet(ctx, tsk)
		if err != nil {
			return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
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

			have, err := a.mp.MPool.MessagesForBlocks(ts.Blocks())
			if err != nil {
				return nil, xerrors.Errorf("getting messages for base ts: %w", err)
			}

			for _, m := range have {
				haveCids[m.Cid()] = struct{}{}
			}
		}

		msgs, err := a.mp.MPool.MessagesForBlocks(ts.Blocks())
		if err != nil {
			return nil, xerrors.Errorf(": %w", err)
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
			return nil, xerrors.Errorf("loading parent tipset: %w", err)
		}
	}
}

func (a *MessagePoolAPI) MpoolClear(ctx context.Context, local bool) error {
	a.mp.MPool.Clear(ctx, local)
	return nil
}

func (a *MessagePoolAPI) MpoolPushUntrusted(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	return a.mp.MPool.PushUntrusted(ctx, smsg)
}

func (a *MessagePoolAPI) MpoolPushMessage(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec) (*types.SignedMessage, error) {
	cp := *msg
	msg = &cp
	inMsg := *msg
	fromA, err := a.mp.chain.API().ResolveToKeyAddr(ctx, msg.From, nil)
	if err != nil {
		return nil, xerrors.Errorf("getting key address: %w", err)
	}
	{
		done, err := a.pushLocks.TakeLock(ctx, fromA)
		if err != nil {
			return nil, xerrors.Errorf("taking lock: %w", err)
		}
		defer done()
	}

	if msg.Nonce != 0 {
		return nil, xerrors.Errorf("MpoolPushMessage expects message nonce to be 0, was %d", msg.Nonce)
	}

	msg, err = a.GasEstimateMessageGas(ctx, msg, spec, types.TipSetKey{})
	if err != nil {
		return nil, xerrors.Errorf("GasEstimateMessageGas error: %w", err)
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
		return nil, xerrors.Errorf("After estimation, GasPremium is greater than GasFeeCap, inmsg: %s, outmsg: %s",
			inJSON, outJSON)
	}

	if msg.From.Protocol() == address.ID {
		log.Warnf("Push from ID address (%s), adjusting to %s", msg.From, fromA)
		msg.From = fromA
	}

	b, err := a.mp.walletAPI.WalletBalance(ctx, msg.From)
	if err != nil {
		return nil, xerrors.Errorf("mpool push: getting origin balance: %w", err)
	}

	if b.LessThan(msg.Value) {
		return nil, xerrors.Errorf("mpool push: not enough funds: %s < %s", b, msg.Value)
	}

	// Sign and push the message
	return a.mp.msgSigner.SignMessage(ctx, msg, func(smsg *types.SignedMessage) error {
		if _, err := a.MpoolPush(ctx, smsg); err != nil {
			return xerrors.Errorf("mpool push: failed to push message: %w", err)
		}
		return nil
	})
}

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

func (a *MessagePoolAPI) MpoolBatchPushMessage(ctx context.Context, msgs []*types.UnsignedMessage, spec *types.MessageSendSpec) ([]*types.SignedMessage, error) {
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

func (a *MessagePoolAPI) MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error) {
	return a.mp.MPool.GetNonce(ctx, addr, types.EmptyTSK)
}

func (a *MessagePoolAPI) MpoolSub(ctx context.Context) (<-chan messagepool.MpoolUpdate, error) {
	return a.mp.MPool.Updates(ctx)
}

func (a *MessagePoolAPI) SendMsg(ctx context.Context, from, to address.Address, method abi.MethodNum, value, maxFee abi.TokenAmount, params []byte) (cid.Cid, error) {
	msg := types.UnsignedMessage{
		To:     to,
		From:   from,
		Value:  value,
		Method: method,
		Params: params,
	}

	smsg, err := a.MpoolPushMessage(ctx, &msg, &types.MessageSendSpec{MaxFee: maxFee})
	if err != nil {
		return cid.Undef, err
	}

	return smsg.Cid(), nil
}

func (a *MessagePoolAPI) GasEstimateMessageGas(ctx context.Context, msg *types.UnsignedMessage, spec *types.MessageSendSpec, tsk types.TipSetKey) (*types.UnsignedMessage, error) {
	return a.mp.MPool.GasEstimateMessageGas(ctx, &types.EstimateMessage{Msg: msg, Spec: spec}, tsk)
}

func (a *MessagePoolAPI) GasBatchEstimateMessageGas(ctx context.Context, estimateMessages []*types.EstimateMessage, fromNonce uint64, tsk types.TipSetKey) ([]*types.EstimateResult, error) {
	return a.mp.MPool.GasBatchEstimateMessageGas(ctx, estimateMessages, fromNonce, tsk)
}

func (a *MessagePoolAPI) GasEstimateFeeCap(ctx context.Context, msg *types.UnsignedMessage, maxqueueblks int64, tsk types.TipSetKey) (big.Int, error) {
	return a.mp.MPool.GasEstimateFeeCap(ctx, msg, maxqueueblks, tsk)
}

func (a *MessagePoolAPI) GasEstimateGasLimit(ctx context.Context, msgIn *types.UnsignedMessage, tsk types.TipSetKey) (int64, error) {
	return a.mp.MPool.GasEstimateGasLimit(ctx, msgIn, tsk)
}

func (a *MessagePoolAPI) GasEstimateGasPremium(ctx context.Context, nblocksincl uint64, sender address.Address, gaslimit int64, tsk types.TipSetKey) (big.Int, error) {
	return a.mp.MPool.GasEstimateGasPremium(ctx, nblocksincl, sender, gaslimit, tsk, a.mp.MPool.PriceCache)
}

func (a *MessagePoolAPI) MpoolCheckMessages(ctx context.Context, protos []*apitypes.MessagePrototype) ([][]apitypes.MessageCheckStatus, error) {
	return a.mp.MPool.CheckMessages(ctx, protos)
}

func (a *MessagePoolAPI) MpoolCheckPendingMessages(ctx context.Context, addr address.Address) ([][]apitypes.MessageCheckStatus, error) {
	return a.mp.MPool.CheckPendingMessages(ctx, addr)
}

func (a *MessagePoolAPI) MpoolCheckReplaceMessages(ctx context.Context, msg []*types.Message) ([][]apitypes.MessageCheckStatus, error) {
	return a.mp.MPool.CheckReplaceMessages(ctx, msg)
}

/*func (a *MessagePoolAPI) WalletSign(ctx context.Context, k address.Address, msg []byte) (*crypto.Signature, error) {
	head := a.mp.chain.ChainReader.GetHead()
	view, err := a.mp.chain.ChainReader.StateView(head)
	if err != nil {
		return nil, err
	}

	keyAddr, err := view.ResolveToKeyAddr(ctx, k)
	if err != nil {
		return nil, xerrors.Errorf("failed to resolve ID address: %v", keyAddr)
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
