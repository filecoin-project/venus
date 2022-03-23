package conformance

import (
	"context"

	"github.com/filecoin-project/venus/pkg/vm/vmcontext"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
)

type fixedRand struct{}

var _ vmcontext.HeadChainRandomness = (*fixedRand)(nil)

// NewFixedRand creates a test vm.Rand that always returns fixed bytes value
// of utf-8 string 'i_am_random_____i_am_random_____'.
func NewFixedRand() vmcontext.HeadChainRandomness {
	return &fixedRand{}
}

func (r *fixedRand) ChainGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return []byte("i_am_random_____i_am_random_____"), nil // 32 bytes.
}

func (r *fixedRand) ChainGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return []byte("i_am_random_____i_am_random_____"), nil // 32 bytes.
}
