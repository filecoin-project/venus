package node

import (
	"context"
	"fmt"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// MessageWaiter provides a method that waits on a chain.Store to contain a
// message in it's longest chain and then run a callback.
type MessageWaiter struct {
	chainReader chain.ReadStore
	cst         *hamt.CborIpldStore
	bs          bstore.Blockstore
}

// NewMessageWaiter returns a new message waiter that can trigger a callback
// when a block with a given message is added into the longest chain of a
// chain reader.
func NewMessageWaiter(chainStore chain.ReadStore, bs bstore.Blockstore, cst *hamt.CborIpldStore) *MessageWaiter {
	return &MessageWaiter{
		chainReader: chainStore,
		cst:         cst,
		bs:          bs,
	}
}

// WaitForMessage searches for a message with Cid, msgCid, then passes it, along with the containing Block and any
// MessageRecipt, to the supplied callback, cb. If an error is encountered, it is returned. Note that it is logically
// possible that an error is returned and the success callback is called. In that case, the error can be safely ignored.
func (node *Node) WaitForMessage(ctx context.Context, msgCid *cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	return node.MessageWaiter.WaitForMessage(ctx, msgCid, cb)
}

// WaitForMessage is the internal implementation of node.WaitForMessage.
// TODO: This implementation will become prohibitively expensive since it involves traversing the entire blockchain.
//       We should replace with an index later.
func (waiter *MessageWaiter) WaitForMessage(ctx context.Context, msgCid *cid.Cid, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) error {
	var emptyErr error
	ctx = log.Start(ctx, "WaitForMessage")
	defer log.Finish(ctx)
	log.Info("Calling WaitForMessage")
	// Ch will contain a stream of blocks to check for message (or errors).
	// Blocks are either in new heaviest tipsets, or next oldest historical blocks.
	ch := make(chan (interface{}))

	// New blocks
	newHeadCh := waiter.chainReader.HeadEvents().Sub(chain.NewHeadTopic)
	defer waiter.chainReader.HeadEvents().Unsub(newHeadCh, chain.NewHeadTopic)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Historical blocks
	historyCh := waiter.chainReader.BlockHistory(ctx)

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

	for raw := range ch {
		switch ts := raw.(type) {
		case error:
			log.Errorf("WaitForMessage: %s", ts)
			return ts
		case consensus.TipSet:
			for _, blk := range ts {
				for _, msg := range blk.Messages {
					c, err := msg.Cid()
					if err != nil {
						log.Errorf("WaitForMessage: %s", err)
						return err
					}
					if c.Equals(msgCid) {
						recpt, err := waiter.receiptFromTipSet(ctx, msgCid, ts)
						if err != nil {
							return errors.Wrap(err, "error retrieving receipt from tipset")
						}
						return cb(blk, msg, recpt)
					}
				}
			}
		}
	}

	return emptyErr
}

// receiptFromTipSet finds the receipt for the message with msgCid in the
// input tipset.  This can differ from the message's receipt as stored in its
// parent block in the case that the message is in conflict with another
// message of the tipset.
func (waiter *MessageWaiter) receiptFromTipSet(ctx context.Context, msgCid *cid.Cid, ts consensus.TipSet) (*types.MessageReceipt, error) {
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
	tsas, err := waiter.chainReader.GetTipSetAndState(ctx, ids.String())
	if err != nil {
		return nil, err
	}
	st, err := state.LoadStateTree(ctx, waiter.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, err
	}
	res, err := consensus.ProcessTipSet(ctx, ts, st, vm.NewStorageMap(waiter.bs))
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
func msgIndexOfTipSet(msgCid *cid.Cid, ts consensus.TipSet, fails types.SortedCidSet) (int, error) {
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
