package events

import (
	"context"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
)

func (me *messageEvents) CheckMsg(ctx context.Context, smsg types.ChainMsg, hnd MsgHandler) CheckFunc {
	msg := smsg.VMMessage()

	return func(ts *types.TipSet) (done bool, more bool, err error) {
		fa, err := me.cs.StateGetActor(ctx, msg.From, ts.Key())
		if err != nil {
			return false, true, err
		}

		// >= because actor nonce is actually the next nonce that is expected to appear on chain
		if msg.Nonce >= fa.Nonce {
			return false, true, nil
		}

		ml, err := me.cs.StateSearchMsg(me.ctx, ts.Key(), msg.Cid(), constants.LookbackNoLimit, true)
		if err != nil {
			return false, true, xerrors.Errorf("getting receipt in CheckMsg: %w", err)
		}

		if ml == nil {
			more, err = hnd(msg, nil, ts, ts.Height())
		} else {
			more, err = hnd(msg, &ml.Receipt, ts, ts.Height())
		}

		return true, more, err
	}
}

func (me *messageEvents) MatchMsg(inmsg *types.UnsignedMessage) MsgMatchFunc {
	return func(msg *types.UnsignedMessage) (matched bool, err error) {
		if msg.From == inmsg.From && msg.Nonce == inmsg.Nonce && !inmsg.Equals(msg) {
			cidTmp := inmsg.Cid()
			return false, xerrors.Errorf("matching msg %s from %s, nonce %d: got duplicate origin/nonce msg %d", cidTmp, inmsg.From, inmsg.Nonce, msg.Nonce)
		}

		return inmsg.Equals(msg), nil
	}
}
