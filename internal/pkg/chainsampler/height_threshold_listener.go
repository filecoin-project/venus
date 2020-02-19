package chainsampler

import (
	"context"

	storage "github.com/filecoin-project/go-storage-miner/apis/node"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// ChainSampler is a function which samples randomness from the chainStore at the
// given height.
type ChainSampler func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error)

// HeightThresholdListener listens for new heaviest chains and notifies when a height threshold is crossed.
type HeightThresholdListener struct {
	target    uint64
	targetHit bool

	seedCh    chan storage.SealSeed
	errCh     chan storage.GetSealSeedError
	invalidCh chan storage.SeedInvalidated
	doneCh    chan storage.FinalityReached
}

// NewHeightThresholdListener creates a new listener
func NewHeightThresholdListener(target uint64, seedCh chan storage.SealSeed, invalidCh chan storage.SeedInvalidated, doneCh chan storage.FinalityReached, errCh chan storage.GetSealSeedError) *HeightThresholdListener {
	return &HeightThresholdListener{
		target:    target,
		targetHit: false,
		seedCh:    seedCh,
		errCh:     errCh,
		invalidCh: invalidCh,
		doneCh:    doneCh,
	}
}

// Handle a chainStore update by sending appropriate status messages back to the channels.
// newChain is all the tipsets that are new since the last head update.
// Normally, this will be a single tipset, but in the case of a re-org it will contain
// all the common ancestors of the new tipset to the greatest common ancestor.
// The tipsets must be ordered from newest (highest block height) to oldest.
// Returns false if this handler is no longer valid.
func (l *HeightThresholdListener) Handle(ctx context.Context, chain []block.TipSet, sampler ChainSampler) (bool, error) {
	if len(chain) < 1 {
		return true, nil
	}

	h, err := chain[0].Height()
	if err != nil {
		return true, err
	}

	// check if we've hit finality and should stop listening
	if h >= l.target+consensus.FinalityEpochs {
		l.doneCh <- storage.FinalityReached{}
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
			l.invalidCh <- storage.SeedInvalidated{}
			l.targetHit = false
			// if we've re-orged to a point before the target
		} else if lcaHeight < l.target {
			l.invalidCh <- storage.SeedInvalidated{}
			err := l.sendRandomness(ctx, chain, sampler)
			if err != nil {
				return true, err
			}
		}
		return true, nil
	}

	// otherwise send randomness if we've hit the height
	if h >= l.target {
		l.targetHit = true
		err := l.sendRandomness(ctx, chain, sampler)
		if err != nil {
			return true, err
		}
	}
	return true, nil
}

func (l *HeightThresholdListener) sendRandomness(ctx context.Context, chain []block.TipSet, sampler ChainSampler) error {
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

	tsHeight, err := firstTargetTipset.Height()
	if err != nil {
		return err
	}

	randomness, err := sampler(ctx, types.NewBlockHeight(tsHeight))
	if err != nil {
		return err
	}

	l.seedCh <- storage.SealSeed{
		BlockHeight: tsHeight,
		TicketBytes: randomness,
	}
	return nil
}
