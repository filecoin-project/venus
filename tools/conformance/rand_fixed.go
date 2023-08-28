package conformance

import (
	"context"

	"github.com/filecoin-project/venus/pkg/vm/vmcontext"

	"github.com/filecoin-project/go-state-types/abi"
)

type fixedRand struct{}

var _ vmcontext.HeadChainRandomness = (*fixedRand)(nil)

// NewFixedRand creates a test vm.Rand that always returns fixed bytes value
// of utf-8 string 'i_am_random_____i_am_random_____'.
func NewFixedRand() vmcontext.HeadChainRandomness {
	return &fixedRand{}
}

var fixedBytes = []byte("i_am_random_____i_am_random_____")

func (r *fixedRand) GetBeaconRandomness(ctx context.Context, round abi.ChainEpoch) ([32]byte, error) {
	return *(*[32]byte)(fixedBytes), nil // 32 bytes.
}

func (r *fixedRand) GetChainRandomness(ctx context.Context, round abi.ChainEpoch) ([32]byte, error) {
	return *(*[32]byte)(fixedBytes), nil // 32 bytes.
}
