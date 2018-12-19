package sectorbuilder

import (
	"sync"
	"time"
)

// SealedSectorPollingInterval defines the interval for which we poll through
// the FFI for sealed sector metadata after adding a piece.
const SealedSectorPollingInterval = 1 * time.Second

// sealStatusPoller is used to poll for sector sealing results.
type sealStatusPoller struct {
	// sectorsAwaitingSealLk protects the sectorsAwaitingSeal set.
	sectorsAwaitingSealLk sync.Mutex

	// sectorsAwaitingSeal is a set of the sector ids awaiting sealing status.
	sectorsAwaitingSeal map[uint64]struct{}

	// stopPollingCh, when sent a value, causes the poller to stop polling.
	stopPollingCh chan struct{}
}

// findSealedSectorMetadataFunc is the type of a function which maps a sector id
// to either an error, nil (if sealing hasn't completed), or a *SealedSectorMetadata (if
// sealing has completed).
type findSealedSectorMetadataFunc = func(uint64) (*SealedSectorMetadata, error)

// newSealStatusPoller initializes and returns an active poller.
func newSealStatusPoller(idsAwaitingSeal []uint64, onSealStatusCh chan SectorSealResult, f findSealedSectorMetadataFunc) *sealStatusPoller {
	p := &sealStatusPoller{
		sectorsAwaitingSealLk: sync.Mutex{},
		sectorsAwaitingSeal:   make(map[uint64]struct{}),
		stopPollingCh:         make(chan struct{}),
	}

	// initialize the sealer with the provided sector ids
	for _, id := range idsAwaitingSeal {
		p.sectorsAwaitingSeal[id] = struct{}{}
	}

	go func() {
		for {
			select {
			case <-p.stopPollingCh:
				return
			default:
				p.sectorsAwaitingSealLk.Lock()

				for id := range p.sectorsAwaitingSeal {
					meta, err := f(id)
					if err != nil {
						onSealStatusCh <- SectorSealResult{
							SectorID:      id,
							SealingErr:    err,
							SealingResult: nil,
						}

						delete(p.sectorsAwaitingSeal, id)
					} else if meta != nil {
						onSealStatusCh <- SectorSealResult{
							SectorID:      id,
							SealingErr:    nil,
							SealingResult: meta,
						}

						delete(p.sectorsAwaitingSeal, id)
					}
				}

				p.sectorsAwaitingSealLk.Unlock()

				time.Sleep(SealedSectorPollingInterval)
			}
		}
	}()

	return p
}

// addSectorID adds the provided sector id to the list of ids whose sealing
// status is being polled for.
func (p *sealStatusPoller) addSectorID(sectorID uint64) {
	p.sectorsAwaitingSealLk.Lock()
	defer p.sectorsAwaitingSealLk.Unlock()

	p.sectorsAwaitingSeal[sectorID] = struct{}{}
}

// stop causes the sealStatusPoller to stop polling. The poller cannot be
// restarted after this method is called.
func (p *sealStatusPoller) stop() {
	p.stopPollingCh <- struct{}{}
}
