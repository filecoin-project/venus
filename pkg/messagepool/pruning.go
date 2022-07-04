package messagepool

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

func (mp *MessagePool) pruneExcessMessages() error {
	mp.curTSLk.Lock()
	ts := mp.curTS
	mp.curTSLk.Unlock()

	mp.lk.Lock()
	defer mp.lk.Unlock()

	if mp.currentSize < mp.cfg.SizeLimitHigh {
		return nil
	}

	select {
	case <-mp.pruneCooldown:
		err := mp.pruneMessages(context.TODO(), ts)
		go func() {
			time.Sleep(mp.cfg.PruneCooldown)
			mp.pruneCooldown <- struct{}{}
		}()
		return err
	default:
		return errors.New("cannot prune before cooldown")
	}
}

func (mp *MessagePool) pruneMessages(ctx context.Context, ts *types.TipSet) error {
	start := time.Now()
	defer func() {
		log.Infof("message pruning took %s", time.Since(start))
	}()

	baseFee, err := mp.api.ChainComputeBaseFee(ctx, ts)
	if err != nil {
		return fmt.Errorf("computing basefee: %v", err)
	}
	baseFeeLowerBound := getBaseFeeLowerBound(baseFee, baseFeeLowerBoundFactor)

	pending, _ := mp.getPendingMessages(ctx, ts, ts)

	// protected actors -- not pruned
	protected := make(map[address.Address]struct{})

	// we never prune priority addresses
	for _, actor := range mp.cfg.PriorityAddrs {
		pk, err := mp.resolveToKey(ctx, actor)
		if err != nil {
			log.Debugf("pruneMessages failed to resolve priority address: %s", err)
		}

		protected[pk] = struct{}{}
	}

	// we also never prune locally published messages
	mp.forEachLocal(ctx, func(ctx context.Context, actor address.Address) {
		protected[actor] = struct{}{}
	})

	// Collect all messages to track which ones to remove and create chains for block inclusion
	pruneMsgs := make(map[cid.Cid]*types.SignedMessage, mp.currentSize)
	keepCount := 0

	var chains []*msgChain
	for actor, mset := range pending {
		// we never prune protected actors
		_, keep := protected[actor]
		if keep {
			keepCount += len(mset)
			continue
		}

		// not a protected actor, track the messages and create chains
		for _, m := range mset {
			pruneMsgs[m.Message.Cid()] = m
		}
		actorChains := mp.createMessageChains(ctx, actor, mset, baseFeeLowerBound, ts)
		chains = append(chains, actorChains...)
	}

	// Sort the chains
	sort.Slice(chains, func(i, j int) bool {
		return chains[i].Before(chains[j])
	})

	// Keep messages (remove them from pruneMsgs) from chains while we are under the low water mark
	loWaterMark := mp.cfg.SizeLimitLow
keepLoop:
	for _, chain := range chains {
		for _, m := range chain.msgs {
			if keepCount < loWaterMark {
				delete(pruneMsgs, m.Message.Cid())
				keepCount++
			} else {
				break keepLoop
			}
		}
	}

	// and remove all messages that are still in pruneMsgs after processing the chains
	log.Infof("Pruning %d messages", len(pruneMsgs))
	for _, m := range pruneMsgs {
		mp.remove(ctx, m.Message.From, m.Message.Nonce, false)
	}

	return nil
}
