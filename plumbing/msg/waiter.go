package msg

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

var log = logging.Logger("messageimpl")

// Waiter waits for a message to appear on chain.
type Waiter struct {
	chainReader chain.ReadStore
	cst         *hamt.CborIpldStore
	bs          bstore.Blockstore
}

// ChainMessage is an on-chain message with its block and receipt.
type ChainMessage struct {
	Message *types.SignedMessage
	Block   *types.Block
	Receipt *types.MessageReceipt
}

// NewWaiter returns a new Waiter.
func NewWaiter(chainStore chain.ReadStore, bs bstore.Blockstore, cst *hamt.CborIpldStore) *Waiter {
	return &Waiter{
		chainReader: chainStore,
		cst:         cst,
		bs:          bs,
	}
}

// Find searches the blockchain history for a message (but doesn't wait).
func (w *Waiter) Find(ctx context.Context, msgCid cid.Cid) (*ChainMessage, bool, error) {
	headTipSetAndState, err := w.chainReader.GetTipSetAndState(ctx, w.chainReader.GetHead())
	if err != nil {
		return nil, false, err
	}
	return w.findMessage(ctx, &headTipSetAndState.TipSet, msgCid)
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
func (w *Waiter) Wait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	ctx = log.Start(ctx, "Waiter.Wait")
	defer log.Finish(ctx)
	log.Infof("Calling Waiter.Wait CID: %s", msgCid.String())

	// Check old blocks
	chainMsg, found, err := w.Find(ctx, msgCid)
	if err != nil {
		return err
	}
	if found {
		return cb(chainMsg.Block, chainMsg.Message, chainMsg.Receipt)
	}

	// Check new blocks
	ch := w.chainReader.HeadEvents().Sub(chain.NewHeadTopic)
	defer w.chainReader.HeadEvents().Unsub(ch, chain.NewHeadTopic)
	chainMsg, found, err = w.waitForMessage(ctx, ch, msgCid)
	if found {
		return cb(chainMsg.Block, chainMsg.Message, chainMsg.Receipt)
	}
	return err
}

// findMessage looks for a message CID in the chain and returns the message,
// block and receipt, when it is found. Reads until the channel is closed or the
// context done. Returns the found message/block (or nil if the channel closed
// without finding it), whether it was found, or an error.
func (w *Waiter) findMessage(ctx context.Context, ts *types.TipSet, msgCid cid.Cid) (*ChainMessage, bool, error) {
	var err error
	for ts != nil {
		for _, blk := range *ts {
			for _, msg := range blk.Messages {
				c, err := msg.Cid()
				if err != nil {
					return nil, false, err
				}
				if c.Equals(msgCid) {
					recpt, err := w.receiptFromTipSet(ctx, msgCid, *ts)
					if err != nil {
						return nil, false, errors.Wrap(err, "error retrieving receipt from tipset")
					}
					return &ChainMessage{msg, blk, recpt}, true, nil
				}
			}
		}
		ts, err = ts.GetNext(ctx, w.chainReader)
		if err != nil {
			log.Errorf("Waiter.Wait: %s", err)
			return nil, false, err
		}
	}
	return nil, false, nil
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
			case types.TipSet:
				for _, blk := range raw {
					for _, msg := range blk.Messages {
						c, err := msg.Cid()
						if err != nil {
							return nil, false, err
						}
						if c.Equals(msgCid) {
							recpt, err := w.receiptFromTipSet(ctx, msgCid, raw)
							if err != nil {
								return nil, false, errors.Wrap(err, "error retrieving receipt from tipset")
							}
							return &ChainMessage{msg, blk, recpt}, true, nil
						}
					}
				}
			default:
				return nil, false, fmt.Errorf("unexpected type in channel: %T", raw)
			}
		}
	}
}

// receiptFromTipSet finds the receipt for the message with msgCid in the
// input tipset.  This can differ from the message's receipt as stored in its
// parent block in the case that the message is in conflict with another
// message of the tipset.
func (w *Waiter) receiptFromTipSet(ctx context.Context, msgCid cid.Cid, ts types.TipSet) (*types.MessageReceipt, error) {
	// Receipts always match block if tipset has only 1 member.
	var rcpt *types.MessageReceipt
	blks := ts.ToSlice()
	if len(ts) == 1 {
		b := blks[0]
		// TODO: this should return an error if a receipt doesn't exist.
		// Right now doing so breaks tests because our test helpers
		// don't correctly apply messages when making test chains.
		j, err := msgIndexOfTipSet(msgCid, ts, types.SortedCidSet{})
		if err != nil {
			return nil, err
		}
		if j < len(b.MessageReceipts) {
			rcpt = b.MessageReceipts[j]
		}
		return rcpt, nil
	}

	// Apply all the tipset's messages to determine the correct receipts.
	ids, err := ts.Parents()
	if err != nil {
		return nil, err
	}
	tsas, err := w.chainReader.GetTipSetAndState(ctx, ids)
	if err != nil {
		return nil, err
	}
	st, err := state.LoadStateTree(ctx, w.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, err
	}

	tsHeight, err := ts.Height()
	if err != nil {
		return nil, err
	}
	tsBlockHeight := types.NewBlockHeight(tsHeight)
	ancestors, err := chain.GetRecentAncestors(ctx, tsas.TipSet, w.chainReader, tsBlockHeight, consensus.AncestorRoundsNeeded, sampling.LookbackParameter)
	if err != nil {
		return nil, err
	}

	res, err := consensus.NewDefaultProcessor().ProcessTipSet(ctx, st, vm.NewStorageMap(w.bs), ts, ancestors)
	if err != nil {
		return nil, err
	}

	// If this is a failing conflict message there is no application receipt.
	if res.Failures.Has(msgCid) {
		return nil, nil
	}

	j, err := msgIndexOfTipSet(msgCid, ts, res.Failures)
	if err != nil {
		return nil, err
	}
	// TODO: out of bounds receipt index should return an error.
	if j < len(res.Results) {
		rcpt = res.Results[j].Receipt
	}
	return rcpt, nil
}

// msgIndexOfTipSet returns the order in which msgCid appears in the canonical
// message ordering of the given tipset, or an error if it is not in the
// tipset.
// TODO: find a better home for this method
func msgIndexOfTipSet(msgCid cid.Cid, ts types.TipSet, fails types.SortedCidSet) (int, error) {
	blks := ts.ToSlice()
	types.SortBlocks(blks)
	var duplicates types.SortedCidSet
	var msgCnt int
	for _, b := range blks {
		for _, msg := range b.Messages {
			c, err := msg.Cid()
			if err != nil {
				return -1, err
			}
			if fails.Has(c) {
				continue
			}
			if duplicates.Has(c) {
				continue
			}
			(&duplicates).Add(c)
			if c.Equals(msgCid) {
				return msgCnt, nil
			}
			msgCnt++
		}
	}

	return -1, fmt.Errorf("message cid %s not in tipset", msgCid.String())
}
