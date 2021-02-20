package conformance

import (
	"context"
	"github.com/filecoin-project/venus/pkg/chain"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
)

type fixedRand struct{}

var _ chain.RandomnessSource = (*fixedRand)(nil)

// NewFixedRand creates a test vm.Rand that always returns fixed bytes value
// of utf-8 string 'i_am_random_____i_am_random_____'.
func NewFixedRand() chain.RandomnessSource {
	return &fixedRand{}
}

func (r *fixedRand) GetRandomnessFromTickets(_ context.Context, _ crypto.DomainSeparationTag, _ abi.ChainEpoch, _ []byte) (abi.Randomness, error) {
	return []byte("i_am_random_____i_am_random_____"), nil // 32 bytes.
}

func (r *fixedRand) GetRandomnessFromBeacon(_ context.Context, _ crypto.DomainSeparationTag, _ abi.ChainEpoch, _ []byte) (abi.Randomness, error) {
	return []byte("i_am_random_____i_am_random_____"), nil // 32 bytes.
}
