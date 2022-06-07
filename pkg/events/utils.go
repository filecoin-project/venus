package events

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// CheckMsg convenience function for checking and matching messages
func (me *messageEvents) CheckMsg(smsg types.ChainMsg, hnd MsgHandler) CheckFunc {
	msg := smsg.VMMessage()

	return func(ctx context.Context, ts *types.TipSet) (done bool, more bool, err error) {
		fa, err := me.cs.StateGetActor(ctx, msg.From, ts.Key())
		if err != nil {
			return false, true, err
		}

		// >= because actor nonce is actually the next nonce that is expected to appear on chain
		if msg.Nonce >= fa.Nonce {
			return false, true, nil
		}

		ml, err := me.cs.StateSearchMsg(ctx, ts.Key(), msg.Cid(), constants.LookbackNoLimit, true)
		if err != nil {
			return false, true, fmt.Errorf("getting receipt in CheckMsg: %w", err)
		}

		if ml == nil {
			more, err = hnd(msg, nil, ts, ts.Height())
		} else {
			more, err = hnd(msg, &ml.Receipt, ts, ts.Height())
		}

		return true, more, err
	}
}

//MatchMsg check that a specific message is in a block message
func (me *messageEvents) MatchMsg(inmsg *types.Message) MsgMatchFunc {
	return func(msg *types.Message) (matched bool, err error) {
		if msg.From == inmsg.From && msg.Nonce == inmsg.Nonce && !inmsg.Equals(msg) {
			return false, fmt.Errorf("matching msg %s from %s, nonce %d: got duplicate origin/nonce msg %d", inmsg.Cid(), inmsg.From, inmsg.Nonce, msg.Nonce)
		}

		return inmsg.Equals(msg), nil
	}
}
