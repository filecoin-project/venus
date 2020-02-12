package chainsampler

import (
	"context"

	"github.com/prometheus/common/log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
)

// HeightThresholdScheduler listens for changes to chain height and notifies when the threshold is hit or invalidated
type HeightThresholdScheduler struct {
	newListener     chan *HeightThresholdListener
	cancelListener  chan *HeightThresholdListener
	heightListeners []*HeightThresholdListener
	listenerDone    chan struct{}
	chainStore      *chain.Store
}

// NewHeightThresholdScheduler creates a new scheduler
func NewHeightThresholdScheduler(chainStore *chain.Store) *HeightThresholdScheduler {
	return &HeightThresholdScheduler{
		newListener:    make(chan *HeightThresholdListener),
		cancelListener: make(chan *HeightThresholdListener),
		listenerDone:   make(chan struct{}),
		chainStore:     chainStore,
	}
}

// StartHeightListener starts the scheduler that manages height listeners.
func (m *HeightThresholdScheduler) StartHeightListener(ctx context.Context, htc <-chan interface{}) {
	go func() {
		var previousHead block.TipSet
		for {
			select {
			case <-htc:
				head, err := m.HandleNewTipSet(ctx, previousHead)
				if err != nil {
					log.Warn("failed to handle new tipset")
				} else {
					previousHead = head
				}
			case heightListener := <-m.newListener:
				m.heightListeners = append(m.heightListeners, heightListener)
			case canceled := <-m.cancelListener:
				listeners := []*HeightThresholdListener{}
				for _, l := range listeners {
					if l != canceled {
						listeners = append(listeners, l)
					}
				}
				m.heightListeners = listeners
			case <-m.listenerDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the scheduler.
func (m *HeightThresholdScheduler) Stop() {
	m.listenerDone <- struct{}{}
}

// AddListener adds a new listener for the target height
func (m *HeightThresholdScheduler) AddListener(target uint64) *HeightThresholdListener {
	hc := make(chan block.TipSetKey)
	ec := make(chan error)
	ic := make(chan struct{})
	dc := make(chan struct{})
	listener := NewHeightThresholdListener(target, hc, ec, ic, dc)
	m.newListener <- listener
	return listener
}

// CancelListener stops a listener from listening and sends a message over its done channel
func (m *HeightThresholdScheduler) CancelListener(listener *HeightThresholdListener) {
	m.cancelListener <- listener
	listener.doneCh <- struct{}{}
}

// HandleNewTipSet must be called when the chain head changes.
func (m *HeightThresholdScheduler) HandleNewTipSet(ctx context.Context, previousHead block.TipSet) (block.TipSet, error) {
	newHeadKey := m.chainStore.GetHead()
	newHead, err := m.chainStore.GetTipSet(newHeadKey)
	if err != nil {
		return block.TipSet{}, err
	}

	_, newTips, err := chain.CollectTipsToCommonAncestor(ctx, m.chainStore, previousHead, newHead)
	if err != nil {
		return block.TipSet{}, err
	}

	newListeners := make([]*HeightThresholdListener, len(m.heightListeners))
	for _, listener := range m.heightListeners {
		valid, err := listener.Handle(ctx, newTips)
		if err != nil {
			log.Error("Error checking storage miner chainStore listener", err)
		}

		if valid {
			newListeners = append(newListeners, listener)
		}
	}
	m.heightListeners = newListeners

	return newHead, nil
}
