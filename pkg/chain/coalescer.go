package chain

import (
	"context"
	"time"

	"github.com/filecoin-project/venus/pkg/block"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("chainstore")

// WrapHeadChangeCoalescer wraps a ReorgNotifee with a head change coalescer.
// minDelay is the minimum coalesce delay; when a head change is first received, the coalescer will
//  wait for that long to coalesce more head changes.
// maxDelay is the maximum coalesce delay; the coalescer will not delay delivery of a head change
//  more than that.
// mergeInterval is the interval that triggers additional coalesce delay; if the last head change was
//  within the merge interval when the coalesce timer fires, then the coalesce time is extended
//  by min delay and up to max delay total.
func WrapHeadChangeCoalescer(fn ReorgNotifee, minDelay, maxDelay, mergeInterval time.Duration) ReorgNotifee {
	c := NewHeadChangeCoalescer(fn, minDelay, maxDelay, mergeInterval)
	return c.HeadChange
}

// HeadChangeCoalescer is a stateful reorg notifee which coalesces incoming head changes
// with pending head changes to reduce state computations from head change notifications.
type HeadChangeCoalescer struct {
	notify ReorgNotifee

	ctx    context.Context
	cancel func()

	eventq chan headChange

	revert []*block.TipSet
	apply  []*block.TipSet
}

type headChange struct {
	revert, apply []*block.TipSet
}

// NewHeadChangeCoalescer creates a HeadChangeCoalescer.
func NewHeadChangeCoalescer(fn ReorgNotifee, minDelay, maxDelay, mergeInterval time.Duration) *HeadChangeCoalescer {
	ctx, cancel := context.WithCancel(context.Background())
	c := &HeadChangeCoalescer{
		notify: fn,
		ctx:    ctx,
		cancel: cancel,
		eventq: make(chan headChange),
	}

	go c.background(minDelay, maxDelay, mergeInterval)

	return c
}

// HeadChange is the ReorgNotifee callback for the stateful coalescer; it receives an incoming
// head change and schedules dispatch of a coalesced head change in the background.
func (c *HeadChangeCoalescer) HeadChange(revert, apply []*block.TipSet) error {
	select {
	case c.eventq <- headChange{revert: revert, apply: apply}:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// Close closes the coalescer and cancels the background dispatch goroutine.
// Any further notification will result in an error.
func (c *HeadChangeCoalescer) Close() error {
	select {
	case <-c.ctx.Done():
	default:
		c.cancel()
	}

	return nil
}

// Implementation details

func (c *HeadChangeCoalescer) background(minDelay, maxDelay, mergeInterval time.Duration) {
	var timerC <-chan time.Time
	var first, last time.Time

	for {
		select {
		case evt := <-c.eventq:
			c.coalesce(evt.revert, evt.apply)

			now := time.Now()
			last = now
			if first.IsZero() {
				first = now
			}

			if timerC == nil {
				timerC = time.After(minDelay)
			}

		case now := <-timerC:
			sinceFirst := now.Sub(first)
			sinceLast := now.Sub(last)

			if sinceLast < mergeInterval && sinceFirst < maxDelay {
				// coalesce some more
				maxWait := maxDelay - sinceFirst
				wait := minDelay
				if maxWait < wait {
					wait = maxWait
				}

				timerC = time.After(wait)
			} else {
				// dispatch
				c.dispatch()

				first = time.Time{}
				last = time.Time{}
				timerC = nil
			}

		case <-c.ctx.Done():
			if c.revert != nil || c.apply != nil {
				c.dispatch()
			}
			return
		}
	}
}

func (c *HeadChangeCoalescer) coalesce(revert, apply []*block.TipSet) {
	// newly reverted tipsets cancel out with pending applys.
	// similarly, newly applied tipsets cancel out with pending reverts.

	// pending tipsets
	pendRevert := make(map[string]struct{}, len(c.revert))
	for _, ts := range c.revert {
		pendRevert[ts.Key().String()] = struct{}{}
	}

	pendApply := make(map[string]struct{}, len(c.apply))
	for _, ts := range c.apply {
		pendApply[ts.Key().String()] = struct{}{}
	}

	// incoming tipsets
	reverting := make(map[string]struct{}, len(revert))
	for _, ts := range revert {
		reverting[ts.Key().String()] = struct{}{}
	}

	applying := make(map[string]struct{}, len(apply))
	for _, ts := range apply {
		applying[ts.Key().String()] = struct{}{}
	}

	// coalesced revert set
	// - pending reverts are cancelled by incoming applys
	// - incoming reverts are cancelled by pending applys
	newRevert := make([]*block.TipSet, 0, len(c.revert)+len(revert))
	for _, ts := range c.revert {
		_, cancel := applying[ts.Key().String()]
		if cancel {
			continue
		}

		newRevert = append(newRevert, ts)
	}

	for _, ts := range revert {
		_, cancel := pendApply[ts.Key().String()]
		if cancel {
			continue
		}

		newRevert = append(newRevert, ts)
	}

	// coalesced apply set
	// - pending applys are cancelled by incoming reverts
	// - incoming applys are cancelled by pending reverts
	newApply := make([]*block.TipSet, 0, len(c.apply)+len(apply))
	for _, ts := range c.apply {
		_, cancel := reverting[ts.Key().String()]
		if cancel {
			continue
		}

		newApply = append(newApply, ts)
	}

	for _, ts := range apply {
		_, cancel := pendRevert[ts.Key().String()]
		if cancel {
			continue
		}

		newApply = append(newApply, ts)
	}

	// commit the coalesced sets
	c.revert = newRevert
	c.apply = newApply
}

func (c *HeadChangeCoalescer) dispatch() {
	err := c.notify(c.revert, c.apply)
	if err != nil {
		log.Errorf("error dispatching coalesced head change notification: %s", err)
	}

	c.revert = nil
	c.apply = nil
}
