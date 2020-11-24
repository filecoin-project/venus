package messaging

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/message"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/moresync"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"
	"time"
)

var MsgNotfound = xerrors.New("message not found")

type MessagingAPI struct { //nolint
	messaging *MessagingSubmodule
}

func (messagingAPI *MessagingAPI) MessagePoolWait(ctx context.Context, messageCount uint) ([]*types.SignedMessage, error) {
	pending := messagingAPI.messaging.MsgPool.Pending()
	for len(pending) < int(messageCount) {
		// Poll pending again after subscribing in case a message arrived since.
		pending = messagingAPI.messaging.MsgPool.Pending()
		time.Sleep(200 * time.Millisecond)
	}

	return pending, nil
}

func (messagingAPI *MessagingAPI) MessageWaitDone(ctx context.Context, msgCid cid.Cid) (*types.MessageReceipt, error) {
	messagingLogger.Infof("Calling Waiter.Wait CID: %s", msgCid.String())

	l := moresync.NewLatch(1)
	var ret *types.MessageReceipt
	cb := func(_ *block.Block, _ types.ChainMsg, rcpt *types.MessageReceipt) error {
		ret = rcpt
		l.Done()
		return nil
	}

	pred := func(msg *types.UnsignedMessage, c cid.Cid) bool {
		return c.Equals(msgCid)
	}

	err := messagingAPI.messaging.Waiter.WaitPredicate(ctx, constants.DefaultConfidence, constants.DefaultMessageWaitLookback, pred, cb)

	if err != nil {
		return nil, err
	}
	l.Wait()
	return ret, nil
}

// OutboxQueues lists addresses with non-empty outbox queues (in no particular order).
func (messagingAPI *MessagingAPI) OutboxQueues() []address.Address {
	return messagingAPI.messaging.Outbox.Queue().Queues()
}

// OutboxQueueLs lists messages in the queue for an address.
func (messagingAPI *MessagingAPI) OutboxQueueLs(sender address.Address) []*message.Queued {
	return messagingAPI.messaging.Outbox.Queue().List(sender)
}

// OutboxQueueClear clears messages in the queue for an address/
func (messagingAPI *MessagingAPI) OutboxQueueClear(ctx context.Context, sender address.Address) error {
	messagingAPI.messaging.Outbox.Queue().Clear(ctx, sender)
	return nil
}

// MessagePoolPending lists messages un-mined in the pool
func (messagingAPI *MessagingAPI) MessagePoolPending() []*types.SignedMessage {
	return messagingAPI.messaging.MsgPool.Pending()
}

// MessagePoolGet fetches a message from the pool.
func (messagingAPI *MessagingAPI) MessagePoolGet(cid cid.Cid) (*types.SignedMessage, error) {
	msg, ok := messagingAPI.messaging.MsgPool.Get(cid)
	if !ok {
		return nil, MsgNotfound
	}
	return msg, nil
}

// MessagePoolRemove removes a message from the message pool.
func (messagingAPI *MessagingAPI) MessagePoolRemove(cid cid.Cid) {
	messagingAPI.messaging.MsgPool.Remove(cid)
}

// MessagePreview previews the Gas cost of a message by running it locally on the client and
// recording the amount of Gas used.
func (messagingAPI *MessagingAPI) MessagePreview(ctx context.Context, from, to address.Address, method abi.MethodNum, params ...interface{}) (types.Unit, error) {
	return messagingAPI.messaging.Previewer.Preview(ctx, from, to, method, params...)
}

// MessageSend sends a message. It uses the default from address if none is given and signs the
// message using the wallet. This call "sends" in the sense that it enqueues the
// message in the msg pool and broadcasts it to the network; it does not wait for the
// message to go on chain. Note that no default from address is provided.  The error
// channel returned receives either nil or an error and is immediately closed after
// the message is published to the network to signal that the publish is complete.
func (messagingAPI *MessagingAPI) MessageSend(ctx context.Context, from, to address.Address, value types.AttoFIL, baseFee types.AttoFIL, gasPremium types.AttoFIL, gasLimit types.Unit, method abi.MethodNum, params interface{}) (cid.Cid, error) {
	msgCid, pubCh, err := messagingAPI.messaging.Outbox.Send(ctx, from, to, value, baseFee, gasPremium, gasLimit, true, method, params)
	if err != nil {
		return cid.Undef, nil
	}
	err = <-pubCh
	if err != nil {
		return cid.Undef, nil
	}
	return msgCid, nil
}

//SignedMessageSend sends a siged message.
func (messagingAPI *MessagingAPI) SignedMessageSend(ctx context.Context, smsg *types.SignedMessage) (cid.Cid, error) {
	msgCid, pubCh, err := messagingAPI.messaging.Outbox.SignedSend(ctx, smsg, true)
	if err != nil {
		return cid.Undef, nil
	}
	err = <-pubCh
	if err != nil {
		return cid.Undef, nil
	}
	return msgCid, nil
}

// MessageWait invokes the callback when a message with the given cid appears on chain.
// It will find the message in both the case that it is already on chain and
// the case that it appears in a newly mined block. An error is returned if one is
// encountered or if the context is canceled. Otherwise, it waits forever for the message
// to appear on chain.
func (messagingAPI *MessagingAPI) MessageWait(ctx context.Context, msgCid cid.Cid, confidence, lookback uint64, cb func(*block.Block, types.ChainMsg, *types.MessageReceipt) error) error {
	return messagingAPI.messaging.Waiter.Wait(ctx, msgCid, confidence, lookback, cb)
}
