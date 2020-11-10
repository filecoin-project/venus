package msg

import (
	"context"
	"fmt"

	"github.com/cskr/pubsub"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm/state"
)

var log = logging.Logger("messageimpl")

var DefaultMessageWaitLookback uint64 = 2 // in most cases, this should be enough to avoid races.

// Abstracts over a store of blockchain state.
type waiterChainReader interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
	GetTipSetReceiptsRoot(block.TipSetKey) (cid.Cid, error)
	HeadEvents() *pubsub.PubSub
}

// Waiter waits for a message to appear on chain.
type Waiter struct {
	chainReader     waiterChainReader
	messageProvider chain.MessageProvider
	cst             cbor.IpldStore
	bs              bstore.Blockstore
}

// ChainMessage is an on-chain message with its block and receipt.
type ChainMessage struct {
	Message *types.SignedMessage
	Block   *block.Block
	Receipt *types.MessageReceipt
}

// WaitPredicate is a function that identifies a message and returns true when found.
type WaitPredicate func(msg *types.SignedMessage, msgCid cid.Cid) bool

// NewWaiter returns a new Waiter.
func NewWaiter(chainStore waiterChainReader, messages chain.MessageProvider, bs bstore.Blockstore, cst cbor.IpldStore) *Waiter {
	return &Waiter{
		chainReader:     chainStore,
		cst:             cst,
		bs:              bs,
		messageProvider: messages,
	}
}

// Find searches the blockchain history (but doesn't wait).
func (w *Waiter) Find(ctx context.Context, lookback uint64, pred WaitPredicate) (*ChainMessage, bool, error) {
	headTipSet, err := w.chainReader.GetTipSet(w.chainReader.GetHead())
	if err != nil {
		return nil, false, err
	}
	return w.findMessage(ctx, headTipSet, lookback, pred)
}

// WaitPredicate invokes the callback when the passed predicate succeeds.
// See api description.
//
// Note: this method does too much -- the callback should just receive the tipset
// containing the message and the caller should pull the receipt out of the block
// if in fact that's what it wants to do, using something like receiptFromTipset.
// Something like receiptFromTipset is necessary because not every message in
// a block will have a receipt in the tipset: it might be a duplicate message.
// This method will always check for the message in the current head tipset.
// A lookback parameter > 1 will cause this method to check for the message in
// up to that many previous tipsets on the chain of the current head.
func (w *Waiter) WaitPredicate(ctx context.Context, lookback uint64, pred WaitPredicate, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	ch := w.chainReader.HeadEvents().Sub(chain.NewHeadTopic)
	defer func() {
		w.chainReader.HeadEvents().Unsub(ch, chain.NewHeadTopic)
	}()

	head, err := w.chainReader.GetTipSet(w.chainReader.GetHead())
	if err != nil {
		return err
	}

	chainMsg, found, err := w.findMessage(ctx, head, lookback, pred)
	if err != nil {
		return err
	}
	if found {
		return cb(chainMsg.Block, chainMsg.Message, chainMsg.Receipt)
	}

	chainMsg, found, err = w.waitForMessage(ctx, ch, head, pred)
	if err != nil {
		return err
	}
	if found {
		return cb(chainMsg.Block, chainMsg.Message, chainMsg.Receipt)
	}
	return err
}

// Wait uses WaitPredicate to invoke the callback when a message with the given cid appears on chain.
func (w *Waiter) Wait(ctx context.Context, msgCid cid.Cid, lookback uint64, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	log.Infof("Calling Waiter.Wait CID: %s", msgCid.String())

	pred := func(msg *types.SignedMessage, c cid.Cid) bool {
		return c.Equals(msgCid)
	}

	return w.WaitPredicate(ctx, lookback, pred, cb)
}

// findMessage looks for a matching in the chain and returns the message,
// block and receipt, when it is found. Returns the found message/block or nil
// if now block with the given CID exists in the chain.
// The lookback parameter is the number of tipsets in the past this method will check before giving up.
func (w *Waiter) findMessage(ctx context.Context, head *block.TipSet, lookback uint64, pred WaitPredicate) (*ChainMessage, bool, error) {
	var err error
	for iterator := chain.IterAncestors(ctx, w.chainReader, head); err == nil && !iterator.Complete(); err = iterator.Next() {
		msg, found, err := w.receiptForTipset(ctx, iterator.Value(), pred)
		if err != nil {
			log.Errorf("Waiter.Wait: %s", err)
			return nil, false, err
		}
		if found {
			return msg, true, nil
		}

		lookback--
		if lookback <= 0 {
			break
		}
	}
	return nil, false, err
}

// waitForMessage looks for a matching message in a channel of tipsets and returns
// the message, block and receipt, when it is found. Reads until the channel is
// closed or the context done. Returns the found message/block (or nil if the
// channel closed without finding it), whether it was found, or an error.
func (w *Waiter) waitForMessage(ctx context.Context, ch <-chan interface{}, head *block.TipSet, pred WaitPredicate) (*ChainMessage, bool, error) {
	lastHead := head
	for {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case raw, more := <-ch:
			if !more {
				return nil, false, nil
			}
			switch raw := raw.(type) {
			case error:
				e := raw.(error)
				log.Errorf("Waiter.Wait: %s", e)
				return nil, false, e
			case *block.TipSet:
				msg, found, err := w.receiptForChain(ctx, raw, lastHead, pred)
				if err != nil {
					return nil, false, err
				}
				if found {
					return msg, found, nil
				}
				lastHead = raw
				// otherwise continue waiting
			default:
				return nil, false, fmt.Errorf("unexpected type in channel: %T", raw)
			}
		}
	}
}

func (w *Waiter) receiptForChain(ctx context.Context, ts *block.TipSet, prevTs *block.TipSet, pred WaitPredicate) (*ChainMessage, bool, error) {
	// New tipsets typically have the previous head as a parent, so handle this cheap case
	parents, err := ts.Parents()
	if err != nil {
		return nil, false, err
	}

	if parents.Equals(prevTs.Key()) {
		return w.receiptForTipset(ctx, ts, pred)
	}

	// check all tipsets up to the last common ancestor of the last tipset we have seen
	_, newChain, err := chain.CollectTipsToCommonAncestor(ctx, w.chainReader, prevTs, ts)
	if err != nil {
		return nil, false, err
	}

	for _, ts := range newChain {
		msg, found, err := w.receiptForTipset(ctx, ts, pred)
		if err != nil {
			return nil, false, err
		}
		if found {
			return msg, found, nil
		}
	}
	return nil, false, nil
}

func (w *Waiter) receiptForTipset(ctx context.Context, ts *block.TipSet, pred WaitPredicate) (*ChainMessage, bool, error) {
	// The targetMsg might be the CID of either a signed SECP message or an unsigned
	// BLS message.
	// This accumulates the CIDs of the messages as they appear on chain (signed or unsigned)
	// but then unwraps them all, obtaining the CID of the unwrapped SECP message body if
	// applicable. This unwrapped message CID is then used to find the target message in the
	// unwrapped de-duplicated tipset messages, and thence the corresponding receipt by index.
	tsMessages := make([][]*types.UnsignedMessage, ts.Len())
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := w.messageProvider.LoadMetaMessages(ctx, blk.Messages.Cid)
		if err != nil {
			return nil, false, err
		}

		originalCids := make([]cid.Cid, len(blsMsgs)+len(secpMsgs))
		unwrappedMsgs := make([]*types.UnsignedMessage, len(blsMsgs)+len(secpMsgs))
		wrappedMsgs := make([]*types.SignedMessage, len(blsMsgs)+len(secpMsgs))
		for j, msg := range blsMsgs {
			c, err := msg.Cid()
			if err != nil {
				return nil, false, err
			}
			originalCids[j] = c
			unwrappedMsgs[j] = msg
			wrappedMsgs[j] = &types.SignedMessage{Message: *msg}
		}
		for j, msg := range secpMsgs {
			c, err := msg.Cid()
			if err != nil {
				return nil, false, err
			}
			originalCids[len(blsMsgs)+j] = c
			unwrappedMsgs[len(blsMsgs)+j] = &msg.Message // Unwrap
			wrappedMsgs[len(blsMsgs)+j] = msg
		}
		tsMessages[i] = unwrappedMsgs

		for k, wrapped := range wrappedMsgs {
			if pred(wrapped, originalCids[k]) {
				// Take CID of the unwrapped message, which might be different from the original.
				unwrappedTarget, err := wrapped.Message.Cid()
				if err != nil {
					return nil, false, err
				}

				recpt, err := w.receiptByIndex(ctx, ts.Key(), unwrappedTarget, tsMessages)
				if err != nil {
					return nil, false, errors.Wrap(err, "error retrieving receipt from tipset")
				}
				return &ChainMessage{wrappedMsgs[k], blk, recpt}, true, nil
			}
		}
	}
	return nil, false, nil
}

func (w *Waiter) receiptByIndex(ctx context.Context, tsKey block.TipSetKey, targetCid cid.Cid, messages [][]*types.UnsignedMessage) (*types.MessageReceipt, error) {
	receiptCid, err := w.chainReader.GetTipSetReceiptsRoot(tsKey)
	if err != nil {
		return nil, err
	}

	receipts, err := w.messageProvider.LoadReceipts(ctx, receiptCid)
	if err != nil {
		return nil, err
	}

	deduped, err := deduppedMessages(messages)
	if err != nil {
		return nil, err
	}

	receiptIndex := 0
	for _, blkMessages := range deduped {
		for _, msg := range blkMessages {
			msgCid, err := msg.Cid()
			if err != nil {
				return nil, err
			}

			if msgCid.Equals(targetCid) {
				if receiptIndex >= len(receipts) {
					return nil, errors.Errorf("could not find message receipt at index %d", receiptIndex)
				}
				return &receipts[receiptIndex], nil
			}
			receiptIndex++
		}
	}
	return nil, errors.Errorf("could not find message cid %s in dedupped messages", targetCid.String())
}

func deduppedMessages(tsMessages [][]*types.UnsignedMessage) ([][]*types.UnsignedMessage, error) {
	allMessages := make([][]*types.UnsignedMessage, len(tsMessages))
	msgFilter := make(map[cid.Cid]struct{})

	for i, blkMessages := range tsMessages {
		for _, msg := range blkMessages {
			mCid, err := msg.Cid()
			if err != nil {
				return nil, err
			}

			_, found := msgFilter[mCid]
			if !found {
				allMessages[i] = append(allMessages[i], msg)
				msgFilter[mCid] = struct{}{}
			}
		}
	}
	return allMessages, nil
}
