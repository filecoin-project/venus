package chainsampler

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
)

var log = logging.Logger("chainsampler") // nolint: deadcode

// HeightThresholdScheduler listens for changes to chain height and notifies when the threshold is hit or invalidated
type HeightThresholdScheduler struct {
	mtx             sync.Mutex
	heightListeners []*HeightThresholdListener
	chainStore      *chain.Store
	prevHead        *block.TipSet
}

// NewHeightThresholdScheduler creates a new scheduler
func NewHeightThresholdScheduler(chainStore *chain.Store) *HeightThresholdScheduler {
	return &HeightThresholdScheduler{
		chainStore: chainStore,
	}
}

// AddListener adds a new listener for the target height
func (hts *HeightThresholdScheduler) AddListener(target abi.ChainEpoch) *HeightThresholdListener {
	hc := make(chan block.TipSetKey)
	ec := make(chan error)
	ic := make(chan struct{})
	dc := make(chan struct{})
	newListener := NewHeightThresholdListener(target, hc, ec, ic, dc)

	hts.mtx.Lock()
	defer hts.mtx.Unlock()
	hts.heightListeners = append(hts.heightListeners, newListener)
	return newListener
}

// CancelListener stops a listener from listening and sends a message over its done channel
func (hts *HeightThresholdScheduler) CancelListener(cancelledListener *HeightThresholdListener) {
	hts.mtx.Lock()
	defer hts.mtx.Unlock()
	var remainingListeners []*HeightThresholdListener
	for _, l := range hts.heightListeners {
		if l != cancelledListener {
			remainingListeners = append(remainingListeners, l)
		}
	}
	hts.heightListeners = remainingListeners
	cancelledListener.DoneCh <- struct{}{}
}

// HandleNewTipSet must be called when the chain head changes.
func (hts *HeightThresholdScheduler) HandleNewTipSet(ctx context.Context, newHead *block.TipSet) error {
	var err error
	var newTips []*block.TipSet

	hts.mtx.Lock()
	defer hts.mtx.Unlock()
	if hts.prevHead.Defined() {
		_, newTips, err = chain.CollectTipsToCommonAncestor(ctx, hts.chainStore, hts.prevHead, newHead)
		if err != nil {
			return errors.Wrapf(err, "failed to collect tips between %s and %s", hts.prevHead, newHead)
		}
	} else {
		newTips = []*block.TipSet{newHead}
	}
	hts.prevHead = newHead

	var newListeners []*HeightThresholdListener
	for _, listener := range hts.heightListeners {
		valid, err := listener.Handle(newTips)
		if err != nil {
			log.Error("Error checking storage miner chainStore listener", err)
		}

		if valid {
			newListeners = append(newListeners, listener)
		}
	}
	hts.heightListeners = newListeners
	return nil
}
