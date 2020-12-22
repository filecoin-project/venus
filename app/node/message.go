package node

import (
	"context"
	"github.com/filecoin-project/venus/pkg/messagepool"

	"github.com/filecoin-project/go-address"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/types"
)

func (node *Node) validateLocalMessage(ctx context.Context, msg pubsub.Message) error {
	m := &types.SignedMessage{}
	if err := m.Unmarshal(msg.GetData()); err != nil {
		return err
	}

	if m.ChainLength() > 32*1024 {
		log.Warnf("local message is too large! (%dB)", m.ChainLength())
		return xerrors.Errorf("local message is too large! (%dB)", m.ChainLength())
	}

	if m.Message.To == address.Undef {
		log.Warn("local message has invalid destination address")
		return xerrors.New("local message has invalid destination address")
	}

	if !m.Message.Value.LessThan(crypto.TotalFilecoinInt) {
		log.Warnf("local messages has too high value: %s", m.Message.Value)
		return xerrors.New("value-too-high")
	}

	if err := node.Mpool.MPool.VerifyMsgSig(m); err != nil {
		log.Warnf("signature verification failed for local message: %s", err)
		return xerrors.Errorf("verify-sig: %s", err)
	}

	return nil
}

func (node *Node) processMessage(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	sender := pubSubMsg.GetSender()

	// ignore messages from self
	if sender == node.Host().ID() {
		return node.validateLocalMessage(ctx, pubSubMsg)
	}

	m := &types.SignedMessage{}
	if err := m.Unmarshal(pubSubMsg.GetData()); err != nil {
		return err
	}

	if err := node.Mpool.MPool.Add(m); err != nil {
		log.Debugf("failed to add message from network to message pool (From: %s, To: %s, Nonce: %d, Value: %s): %s", m.Message.From, m.Message.To, m.Message.Nonce, types.FIL(m.Message.Value), err)
		switch {
		case xerrors.Is(err, messagepool.ErrSoftValidationFailure):
			fallthrough
		case xerrors.Is(err, messagepool.ErrRBFTooLowPremium):
			fallthrough
		case xerrors.Is(err, messagepool.ErrTooManyPendingMessages):
			fallthrough
		case xerrors.Is(err, messagepool.ErrNonceGap):
			fallthrough
		case xerrors.Is(err, messagepool.ErrNonceTooLow):
			return nil
		default:
			return err
		}
	}
	return err
}
