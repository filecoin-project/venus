package conformance

import (
	"bytes"
	"context"

	"github.com/filecoin-project/venus/pkg/vm/vmcontext"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/test-vectors/schema"
)

type ReplayingRand struct {
	reporter Reporter
	recorded schema.Randomness
	fallback vmcontext.HeadChainRandomness
}

var _ vmcontext.HeadChainRandomness = (*ReplayingRand)(nil)

// NewReplayingRand replays recorded randomness when requested, falling back to
// fixed randomness if the value cannot be found; hence this is a safe
// backwards-compatible replacement for fixedRand.
func NewReplayingRand(reporter Reporter, recorded schema.Randomness) *ReplayingRand {
	return &ReplayingRand{
		reporter: reporter,
		recorded: recorded,
		fallback: NewFixedRand(),
	}
}

func (r *ReplayingRand) match(requested schema.RandomnessRule) ([]byte, bool) {
	for _, other := range r.recorded {
		if other.On.Kind == requested.Kind &&
			other.On.Epoch == requested.Epoch &&
			other.On.DomainSeparationTag == requested.DomainSeparationTag &&
			bytes.Equal(other.On.Entropy, requested.Entropy) {
			return other.Return, true
		}
	}
	return nil, false
}

func (r *ReplayingRand) ChainGetRandomnessFromBeacon(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return r.getBeaconRandomness(ctx, pers, round, entropy, false)
}

func (r *ReplayingRand) ChainGetRandomnessFromTickets(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return r.getChainRandomness(ctx, pers, round, entropy, false)
}

func (r *ReplayingRand) getChainRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte, lookback bool) ([]byte, error) {
	rule := schema.RandomnessRule{
		Kind:                schema.RandomnessChain,
		DomainSeparationTag: int64(pers),
		Epoch:               int64(round),
		Entropy:             entropy,
	}

	if ret, ok := r.match(rule); ok {
		r.reporter.Logf("returning saved chain randomness: dst=%d, epoch=%d, entropy=%x, result=%x", pers, round, entropy, ret)
		return ret, nil
	}

	r.reporter.Logf("returning fallback chain randomness: dst=%d, epoch=%d, entropy=%x", pers, round, entropy)

	return r.fallback.ChainGetRandomnessFromTickets(ctx, pers, round, entropy)
}

func (r *ReplayingRand) getBeaconRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte, lookback bool) ([]byte, error) {
	rule := schema.RandomnessRule{
		Kind:                schema.RandomnessBeacon,
		DomainSeparationTag: int64(pers),
		Epoch:               int64(round),
		Entropy:             entropy,
	}

	if ret, ok := r.match(rule); ok {
		r.reporter.Logf("returning saved beacon randomness: dst=%d, epoch=%d, entropy=%x, result=%x", pers, round, entropy, ret)
		return ret, nil
	}

	r.reporter.Logf("returning fallback beacon randomness: dst=%d, epoch=%d, entropy=%x", pers, round, entropy)

	return r.fallback.ChainGetRandomnessFromBeacon(ctx, pers, round, entropy)
}
