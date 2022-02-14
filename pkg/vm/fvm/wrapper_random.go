package vm

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/vm"
)

type Rand interface {
	GetChainRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
}

var _ Rand = (*wrapperRand)(nil)

type wrapperRand struct {
	vm.ChainRandomness
}

func newWrapperRand(r vm.ChainRandomness) Rand {
	return wrapperRand{ChainRandomness: r}
}

func (r wrapperRand) GetChainRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.ChainGetRandomnessFromTickets(ctx, pers, round, entropy)
}

func (r wrapperRand) GetBeaconRandomness(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.ChainGetRandomnessFromBeacon(ctx, pers, round, entropy)
}
