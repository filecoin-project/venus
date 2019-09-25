package storage

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/util/moresync"
)

// FakeProver provides fake PoSt proofs for a miner.
type FakeProver struct {
	// If non-nil, CalculatePoSt calls done when invoked.
	postStarted *moresync.Latch
	// If non-nil, CalculatePoSt waits for this latch to be released.
	postDone *moresync.Latch
}

// CalculatePoSt returns a fixed fake proof.
func (p *FakeProver) CalculatePoSt(ctx context.Context, start, end *types.BlockHeight, seed types.PoStChallengeSeed, inputs []PoStInputs) (*PoStSubmission, error) {
	if p.postStarted != nil {
		p.postStarted.Done()
	}

	time.Sleep(10 * time.Millisecond)

	if p.postDone != nil {
		p.postDone.Wait()
	}
	return &PoStSubmission{
		Proof: types.PoStProof(seed[:]), // Return the seed as the proof
	}, nil
}
