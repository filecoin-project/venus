package msg

import (
	"context"
	"fmt"

	"github.com/cskr/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var log = logging.Logger("messageimpl")

// Abstracts over a store of blockchain state.
type waiterChainReader interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (block.TipSet, error)
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
	GetTipSetReceiptsRoot(block.TipSetKey) (cid.Cid, error)
	HeadEvents() *pubsub.PubSub
}

// Waiter waits for a message to appear on chain.
type Waiter struct {
	chainReader     waiterChainReader
	messageProvider chain.MessageProvider
	cst             *hamt.CborIpldStore
	bs              bstore.Blockstore
}

// ChainMessage is an on-chain message with its block and receipt.
type ChainMessage struct {
	Message *types.SignedMessage
	Block   *block.Block
	Receipt *types.MessageReceipt
}

// NewWaiter returns a new Waiter.
func NewWaiter(chainStore waiterChainReader, messages chain.MessageProvider, bs bstore.Blockstore, cst *hamt.CborIpldStore) *Waiter {
	return &Waiter{
		chainReader:     chainStore,
		cst:             cst,
		bs:              bs,
		messageProvider: messages,
	}
}

// Find searches the blockchain history for a message (but doesn't wait).
func (w *Waiter) Find(ctx context.Context, msgCid cid.Cid) (*ChainMessage, bool, error) {
	headTipSet, err := w.chainReader.GetTipSet(w.chainReader.GetHead())
	if err != nil {
		return nil, false, err
	}
	return w.findMessage(ctx, headTipSet, msgCid)
}

// Wait invokes the callback when a message with the given cid appears on chain.
// See api description.
//
// Note: this method does too much -- the callback should just receive the tipset
// containing the message and the caller should pull the receipt out of the block
// if in fact that's what it wants to do, using something like receiptFromTipset.
// Something like receiptFromTipset is necessary because not every message in
// a block will have a receipt in the tipset: it might be a duplicate message.
//
// TODO: This implementation will become prohibitively expensive since it
// traverses the entire chain. We should use an index instead.
// https://github.com/filecoin-project/go-filecoin/issues/1518
func (w *Waiter) Wait(ctx context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	log.Infof("Calling Waiter.Wait CID: %s", msgCid.String())

	ch := w.chainReader.HeadEvents().Sub(chain.NewHeadTopic)
	defer w.chainReader.HeadEvents().Unsub(ch, chain.NewHeadTopic)

	chainMsg, found, err := w.Find(ctx, msgCid)
	if err != nil {
		return err
	}
	if found {
		return cb(chainMsg.Block, chainMsg.Message, chainMsg.Receipt)
	}

	chainMsg, found, err = w.waitForMessage(ctx, ch, msgCid)
	if err != nil {
		return err
	}
	if found {
		return cb(chainMsg.Block, chainMsg.Message, chainMsg.Receipt)
	}
	return err
}

// findMessage looks for a message CID in the chain and returns the message,
// block and receipt, when it is found. Returns the found message/block or nil
// if now block with the given CID exists in the chain.
func (w *Waiter) findMessage(ctx context.Context, ts block.TipSet, msgCid cid.Cid) (*ChainMessage, bool, error) {
	var err error
	for iterator := chain.IterAncestors(ctx, w.chainReader, ts); err == nil && !iterator.Complete(); err = iterator.Next() {
		msg, found, err := w.receiptForTipset(ctx, iterator.Value(), msgCid)
		if err != nil {
			log.Errorf("Waiter.Wait: %s", err)
			return nil, false, err
		}
		if found {
			return msg, true, nil
		}
	}
	return nil, false, err
}

// waitForMessage looks for a message CID in a channel of tipsets and returns
// the message, block and receipt, when it is found. Reads until the channel is
// closed or the context done. Returns the found message/block (or nil if the
// channel closed without finding it), whether it was found, or an error.
func (w *Waiter) waitForMessage(ctx context.Context, ch <-chan interface{}, msgCid cid.Cid) (*ChainMessage, bool, error) {
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
			case block.TipSet:
				msg, found, err := w.receiptForTipset(ctx, raw, msgCid)
				if err != nil {
					return nil, false, err
				}
				if found {
					return msg, found, nil
				}
				// otherwise continue waiting
			default:
				return nil, false, fmt.Errorf("unexpected type in channel: %T", raw)
			}
		}
	}
}

func (w *Waiter) receiptForTipset(ctx context.Context, ts block.TipSet, targetMsg cid.Cid) (*ChainMessage, bool, error) {
	// The targetMsg might be the CID of either a signed SECP message or an unsigned
	// BLS message.
	// This accumulates the CIDs of the messages as they appear on chain (signed or unsigned)
	// but then unwraps them all, obtaining the CID of the unwrapped SECP message body if
	// applicable. This unwrapped message CID is then used to find the target message in the
	// unwrapped de-duplicated tipset messages, and thence the corresponding receipt by index.
	tsMessages := make([][]*types.UnsignedMessage, ts.Len())
	for i := 0; i < ts.Len(); i++ {
		blk := ts.At(i)
		secpMsgs, blsMsgs, err := w.messageProvider.LoadMessages(ctx, blk.Messages)
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

		for k, uwmsg := range unwrappedMsgs {
			if originalCids[k].Equals(targetMsg) {
				// Take CID of the unwrapped message, which might be different from the original.
				unwrappedTarget, err := uwmsg.Cid()
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
	// receiptCid, err := w.chainReader.GetTipSetReceiptsRoot(tsKey)
	// if err != nil {
	// 	return nil, err
	// }

	// receipts, err := w.messageProvider.LoadReceipts(ctx, receiptCid)
	// if err != nil {
	// 	return nil, err
	// }

	// deduped, err := consensus.DeduppedMessages(messages)
	// if err != nil {
	// 	return nil, err
	// }

	// receiptIndex := 0
	// for _, blkMessages := range deduped {
	// 	for _, msg := range blkMessages {
	// 		msgCid, err := msg.Cid()
	// 		if err != nil {
	// 			return nil, err
	// 		}

	// 		if msgCid.Equals(targetCid) {
	// 			if receiptIndex >= len(receipts) {
	// 				return nil, errors.Errorf("could not find message receipt at index %d", receiptIndex)
	// 			}
	// 			return receipts[receiptIndex], nil
	// 		}
	// 		receiptIndex++
	// 	}
	// }
	// return nil, errors.Errorf("could not find message cid %s in dedupped messages", targetCid.String())

	// Dragons: do something
	return nil, fmt.Errorf("re-write or delete")
}
