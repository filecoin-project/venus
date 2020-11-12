package chainsampler

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/specactors/policy"
)

// HeightThresholdListener listens for new heaviest chains and notifies when a height threshold is crossed.
type HeightThresholdListener struct {
	target    abi.ChainEpoch
	targetHit bool

	HitCh     chan block.TipSetKey
	ErrCh     chan error
	InvalidCh chan struct{}
	DoneCh    chan struct{}
}

// NewHeightThresholdListener creates a new listener
func NewHeightThresholdListener(target abi.ChainEpoch, hitCh chan block.TipSetKey, errCh chan error, invalidCh, doneCh chan struct{}) *HeightThresholdListener {
	return &HeightThresholdListener{
		target:    target,
		targetHit: false,
		HitCh:     hitCh,
		ErrCh:     errCh,
		InvalidCh: invalidCh,
		DoneCh:    doneCh,
	}
}

// Handle a chainStore update by sending appropriate status messages back to the channels.
// newChain is all the tipsets that are new since the last head update.
// Normally, this will be a single tipset, but in the case of a re-org it will contain
// all the common ancestors of the new tipset to the greatest common ancestor.
// The tipsets must be ordered from newest (highest block height) to oldest.
// Returns false if this handler is no longer valid.
func (l *HeightThresholdListener) Handle(chain []*block.TipSet) (bool, error) {
	if len(chain) < 1 {
		return true, nil
	}

	h, err := chain[0].Height()
	if err != nil {
		return true, err
	}

	// check if we've hit finality and should stop listening
	if h >= l.target+policy.ChainFinality {
		l.DoneCh <- struct{}{}
		return false, nil
	}

	lcaHeight, err := chain[len(chain)-1].Height()
	if err != nil {
		return true, err
	}

	// if we have already seen a target tipset
	if l.targetHit {
		// if we've completely reverted
		if h < l.target {
			l.InvalidCh <- struct{}{}
			l.targetHit = false
			// if we've re-orged to a point before the target
		} else if lcaHeight < l.target {
			l.InvalidCh <- struct{}{}
			err := l.sendHit(chain)
			if err != nil {
				return true, err
			}
		}
		return true, nil
	}

	// otherwise send randomness if we've hit the height
	if h >= l.target {
		l.targetHit = true
		err := l.sendHit(chain)
		if err != nil {
			return true, err
		}
	}
	return true, nil
}

func (l *HeightThresholdListener) sendHit(chain []*block.TipSet) error {
	// assume chainStore not empty and first tipset height greater than target
	firstTargetTipset := chain[0]
	for _, ts := range chain {
		h, err := ts.Height()
		if err != nil {
			return err
		}

		if h < l.target {
			break
		}
		firstTargetTipset = ts
	}

	l.HitCh <- firstTargetTipset.Key()
	return nil
}
