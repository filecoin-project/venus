package message

import (
	"context"
	"fmt"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

var log = logging.Logger("message.Waiter")

// Waiter waits for a message to appear on chain.
type Waiter struct {
	chainReader chain.ReadStore
	cst         *hamt.CborIpldStore
	bs          bstore.Blockstore
}

// NewWaiter returns a new Waiter.
func NewWaiter(chainStore chain.ReadStore, bs bstore.Blockstore, cst *hamt.CborIpldStore) *Waiter {
	return &Waiter{
		chainReader: chainStore,
		cst:         cst,
		bs:          bs,
	}
}

// Wait invokes the callback when a message with the given cid appears on chain.
// It will find the message in both the case that it is already on chain and
// the case that it appears in a newly mined block. An error is returned if one is
// encountered. It is possible for both an error to be returned and the callback
// to be invoked, eg if an error was encountered trying to find the block
// in the block history but it suddenly appears in a newly mined block. Unless
// the context is canceled this method will block forever if the message never
// appears on chain.
//
// Note: this method does too much -- the callback should just receive the tipset
// containing the message and the caller should pull the receipt out of the block
// if in fact that's what it wants to do, using something like receiptFromTipset.
// Something like receiptFromTipset is necessary because not every message in
// a block will have a receipt in the tipset: it might be a duplicate message.
//
// TODO: This implementation will become prohibitively expensive since it
// traverses the entire chain. We should use an index instead.
func (w *Waiter) Wait(ctx context.Context, msgCid cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	var emptyErr error
	ctx = log.Start(ctx, "Waiter.Wait")
	defer log.Finish(ctx)
	log.Info("Calling Waiter.Wait")
	// Ch will contain a stream of blocks to check for message (or errors).
	// Blocks are either in new heaviest tipsets, or next oldest historical blocks.
	ch := make(chan (interface{}))

	// New blocks
	newHeadCh := w.chainReader.HeadEvents().Sub(chain.NewHeadTopic)
	defer w.chainReader.HeadEvents().Unsub(newHeadCh, chain.NewHeadTopic)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Historical blocks
	historyCh := w.chainReader.BlockHistory(ctx)

	// Merge historical and new block Channels.
	go func() {
		for raw := range newHeadCh {
			ch <- raw
		}
	}()
	go func() {
		for raw := range historyCh {
			ch <- raw
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case raw, more := <-ch:
			if !more {
				return emptyErr
			}
			switch ts := raw.(type) {
			case error:
				log.Errorf("Waiter.Wait: %s", ts)
				return ts
			case consensus.TipSet:
				for _, blk := range ts {
					for _, msg := range blk.Messages {
						c, err := msg.Cid()
						if err != nil {
							log.Errorf("Waiter.Wait: %s", err)
							return err
						}
						if c.Equals(msgCid) {
							recpt, err := w.receiptFromTipSet(ctx, msgCid, ts)
							if err != nil {
								return errors.Wrap(err, "error retrieving receipt from tipset")
							}
							return cb(blk, msg, recpt)
						}
					}
				}
			}

		}
	}
}

// receiptFromTipSet finds the receipt for the message with msgCid in the
// input tipset.  This can differ from the message's receipt as stored in its
// parent block in the case that the message is in conflict with another
// message of the tipset.
func (w *Waiter) receiptFromTipSet(ctx context.Context, msgCid cid.Cid, ts consensus.TipSet) (*types.MessageReceipt, error) {
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
	tsas, err := w.chainReader.GetTipSetAndState(ctx, ids.String())
	if err != nil {
		return nil, err
	}
	st, err := state.LoadStateTree(ctx, w.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, err
	}
	res, err := consensus.ProcessTipSet(ctx, ts, st, vm.NewStorageMap(w.bs))
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
func msgIndexOfTipSet(msgCid cid.Cid, ts consensus.TipSet, fails types.SortedCidSet) (int, error) {
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
