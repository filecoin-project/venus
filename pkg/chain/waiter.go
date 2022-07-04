package chain

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"
)

type MsgLookup struct {
	Message   cid.Cid // Can be different than requested, in case it was replaced, but only gas values changed
	Receipt   types.MessageReceipt
	ReturnDec interface{}
	TipSet    types.TipSetKey
	Height    abi.ChainEpoch
}

// Abstracts over a store of blockchain state.
type waiterChainReader interface {
	GetHead() *types.TipSet
	GetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	LookupID(context.Context, *types.TipSet, address.Address) (address.Address, error)
	GetActorAt(context.Context, *types.TipSet, address.Address) (*types.Actor, error)
	GetTipSetReceiptsRoot(context.Context, *types.TipSet) (cid.Cid, error)
	SubHeadChanges(context.Context) chan []*types.HeadChange
}

type IStmgr interface {
	GetActorAt(context.Context, address.Address, *types.TipSet) (*types.Actor, error)
	RunStateTransition(context.Context, *types.TipSet) (root cid.Cid, receipts cid.Cid, err error)
}

// Waiter waits for a message to appear on chain.
type Waiter struct {
	chainReader     waiterChainReader
	messageProvider MessageProvider
	cst             cbor.IpldStore
	bs              bstore.Blockstore
	Stmgr           IStmgr
}

// WaitPredicate is a function that identifies a message and returns true when found.
type WaitPredicate func(msg *types.Message, msgCid cid.Cid) bool

// NewWaiter returns a new Waiter.
func NewWaiter(chainStore waiterChainReader, messages MessageProvider, bs bstore.Blockstore, cst cbor.IpldStore) *Waiter {
	return &Waiter{
		chainReader:     chainStore,
		cst:             cst,
		bs:              bs,
		messageProvider: messages,
	}
}

// Find searches the blockchain history (but doesn't wait).
func (w *Waiter) Find(ctx context.Context, msg types.ChainMsg, lookback abi.ChainEpoch, ts *types.TipSet, allowReplaced bool) (*types.ChainMessage, bool, error) {
	if ts == nil {
		ts = w.chainReader.GetHead()
	}

	return w.findMessage(ctx, ts, msg, lookback, allowReplaced)
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
func (w *Waiter) WaitPredicate(ctx context.Context, msg types.ChainMsg, confidence uint64, lookback abi.ChainEpoch, allowReplaced bool) (*types.ChainMessage, error) {
	ch := w.chainReader.SubHeadChanges(ctx)
	chainMsg, found, err := w.waitForMessage(ctx, ch, msg, confidence, lookback, allowReplaced)
	if err != nil {
		return nil, err
	}
	if found {
		return chainMsg, nil
	}
	return nil, nil
}

// Wait uses WaitPredicate to invoke the callback when a message with the given cid appears on chain.
func (w *Waiter) Wait(ctx context.Context, msg types.ChainMsg, confidence uint64, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*types.ChainMessage, error) {
	mid := msg.VMMessage().Cid()
	log.Infof("Calling Waiter.Wait CID: %s", mid.String())

	return w.WaitPredicate(ctx, msg, confidence, lookbackLimit, allowReplaced)
}

// findMessage looks for a matching in the chain and returns the message,
// block and receipt, when it is found. Returns the found message/block or nil
// if now block with the given CID exists in the chain.
// The lookback parameter is the number of tipsets in the past this method will check before giving up.
func (w *Waiter) findMessage(ctx context.Context, from *types.TipSet, m types.ChainMsg, lookback abi.ChainEpoch, allowReplaced bool) (*types.ChainMessage, bool, error) {
	limitHeight := from.Height() - lookback
	noLimit := lookback == constants.LookbackNoLimit

	cur := from
	curActor, err := w.Stmgr.GetActorAt(ctx, m.VMMessage().From, cur)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load from actor")
	}

	mNonce := m.VMMessage().Nonce

	for {
		// If we've reached the genesis block, or we've reached the limit of
		// how far back to look
		if cur.Height() == 0 || !noLimit && cur.Height() <= limitHeight {
			// it ain't here!
			return nil, false, nil
		}

		select {
		case <-ctx.Done():
			return nil, false, nil
		default:
		}

		// we either have no messages from the sender, or the latest message we found has a lower nonce than the one being searched for,
		// either way, no reason to lookback, it ain't there
		if curActor == nil || curActor.Nonce == 0 || curActor.Nonce < mNonce {
			return nil, false, nil
		}

		pts, err := w.chainReader.GetTipSet(ctx, cur.Parents())
		if err != nil {
			return nil, false, fmt.Errorf("failed to load tipset during msg wait searchback: %w", err)
		}

		act, err := w.Stmgr.GetActorAt(ctx, m.VMMessage().From, pts)
		actorNoExist := errors.Is(err, types.ErrActorNotFound)
		if err != nil && !actorNoExist {
			return nil, false, fmt.Errorf("failed to load the actor: %w", err)
		}

		// check that between cur and parent tipset the nonce fell into range of our message
		if actorNoExist || (curActor.Nonce > mNonce && act.Nonce <= mNonce) {
			msg, found, err := w.receiptForTipset(ctx, cur, m, allowReplaced)
			if err != nil {
				log.Errorf("Waiter.Wait: %s", err)
				return nil, false, err
			}
			if found {
				return msg, true, nil
			}
		}

		cur = pts
		curActor = act
	}
}

// waitForMessage looks for a matching message in a channel of tipsets and returns
// the message, block and receipt, when it is found. Reads until the channel is
// closed or the context done. Returns the found message/block (or nil if the
// channel closed without finding it), whether it was found, or an error.
// notice matching mesage by message from and nonce. the return message may not be
// expected, because there maybe another message have the same from and nonce value
func (w *Waiter) waitForMessage(ctx context.Context, ch <-chan []*types.HeadChange, msg types.ChainMsg, confidence uint64, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*types.ChainMessage, bool, error) {
	current, ok := <-ch
	if !ok {
		return nil, false, fmt.Errorf("SubHeadChanges stream was invalid")
	}
	//todo message wait
	if len(current) != 1 {
		return nil, false, fmt.Errorf("SubHeadChanges first entry should have been one item")
	}

	if current[0].Type != types.HCCurrent {
		return nil, false, fmt.Errorf("expected current head on SHC stream (got %s)", current[0].Type)
	}

	currentHead := current[0].Val
	chainMsg, found, err := w.receiptForTipset(ctx, currentHead, msg, allowReplaced)
	if err != nil {
		return nil, false, err
	}
	if found {
		return chainMsg, found, nil
	}

	var backRcp *types.ChainMessage
	backSearchWait := make(chan struct{})
	go func() {
		r, foundMsg, err := w.findMessage(ctx, currentHead, msg, lookbackLimit, allowReplaced)
		if err != nil {
			log.Warnf("failed to look back through chain for message: %w", err)
			return
		}
		if foundMsg {
			backRcp = r
			close(backSearchWait)
		}
	}()

	var candidateTS *types.TipSet
	var candidateRcp *types.ChainMessage
	heightOfHead := currentHead.Height()
	reverts := map[string]bool{}

	for {
		select {
		case notif, ok := <-ch:
			if !ok {
				return nil, false, err
			}
			for _, val := range notif {
				switch val.Type {
				case types.HCRevert:
					if val.Val.Equals(candidateTS) {
						candidateTS = nil
						candidateRcp = nil
					}
					if backSearchWait != nil {
						reverts[val.Val.Key().String()] = true
					}
				case types.HCApply:
					if candidateTS != nil && val.Val.Height() >= candidateTS.Height()+abi.ChainEpoch(confidence) {
						return candidateRcp, true, nil
					}

					r, foundMsg, err := w.receiptForTipset(ctx, val.Val, msg, allowReplaced)
					if err != nil {
						return nil, false, err
					}
					if r != nil {
						if confidence == 0 {
							return r, foundMsg, err
						}
						candidateTS = val.Val
						candidateRcp = r
					}
					heightOfHead = val.Val.Height()
				}
			}
		case <-backSearchWait:
			// check if we found the message in the chain and that is hasn't been reverted since we started searching
			if backRcp != nil && !reverts[backRcp.TS.Key().String()] {
				// if head is at or past confidence interval, return immediately
				if heightOfHead >= backRcp.TS.Height()+abi.ChainEpoch(confidence) {
					return backRcp, true, nil
				}

				// wait for confidence interval
				candidateTS = backRcp.TS
				candidateRcp = backRcp
			}
			reverts = nil
			backSearchWait = nil
		case <-ctx.Done():
			return nil, false, err
		}
	}
}

func (w *Waiter) receiptForTipset(ctx context.Context, ts *types.TipSet, msg types.ChainMsg, allowReplaced bool) (*types.ChainMessage, bool, error) {
	// The genesis block
	if ts.Height() == 0 {
		return nil, false, nil
	}

	pts, err := w.chainReader.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, false, err
	}
	blockMessageInfos, err := w.messageProvider.LoadTipSetMessage(ctx, pts)
	if err != nil {
		return nil, false, err
	}
	expectedMsg := msg.VMMessage()
	expectedCid := msg.Cid()
	expectedNonce := msg.VMMessage().Nonce
	expectedFrom := msg.VMMessage().From
	for _, bms := range blockMessageInfos {
		for _, msg := range append(bms.BlsMessages, bms.SecpkMessages...) {
			msgCid := msg.Cid()
			if msg.VMMessage().From == expectedFrom { // cheaper to just check origin first
				if msg.VMMessage().Nonce == expectedNonce {
					if allowReplaced && msg.VMMessage().EqualCall(expectedMsg) {
						if expectedCid != msgCid {
							log.Warnw("found message with equal nonce and call params but different CID",
								"wanted", expectedCid, "found", msgCid, "nonce", expectedNonce, "from", expectedFrom)
						}
						recpt, err := w.receiptByIndex(ctx, pts, msgCid, blockMessageInfos)
						if err != nil {
							return nil, false, errors.Wrap(err, "error retrieving receipt from tipset")
						}
						return &types.ChainMessage{TS: ts, Message: msg.VMMessage(), Block: bms.Block, Receipt: recpt}, true, nil
					}

					// this should be that message
					return nil, false, fmt.Errorf("found message with equal nonce as the one we are looking for (F: n %d, TS: %s n%d)",
						expectedMsg.Nonce, msg.Cid(), msg.VMMessage().Nonce)
				}
			}
		}
	}

	return nil, false, nil
}

func (w *Waiter) receiptByIndex(ctx context.Context, ts *types.TipSet, targetCid cid.Cid, blockMsgs []types.BlockMessagesInfo) (*types.MessageReceipt, error) {
	var receiptCid cid.Cid
	var err error
	if _, receiptCid, err = w.Stmgr.RunStateTransition(ctx, ts); err != nil {
		return nil, fmt.Errorf("RunStateTransition failed:%w", err)
	}

	receipts, err := w.messageProvider.LoadReceipts(ctx, receiptCid)
	if err != nil {
		return nil, err
	}

	receiptIndex := 0
	for _, blkInfo := range blockMsgs {
		//todo aggrate bls and secp msg to one msg
		for _, msg := range append(blkInfo.BlsMessages, blkInfo.SecpkMessages...) {
			if msg.Cid().Equals(targetCid) {
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
