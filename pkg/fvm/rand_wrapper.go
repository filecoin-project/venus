package fvm

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/vm"
)

type Rand interface {
	GetChainRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
}

var _ Rand = (*wrapperRand)(nil)

type wrapperRand struct {
	vm.ChainRandomness
}

func NewWrapperRand(r vm.ChainRandomness) Rand {
	return wrapperRand{ChainRandomness: r}
}

func (r wrapperRand) GetChainRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.ChainGetRandomnessFromTickets(ctx, pers, round, entropy)
}

func (r wrapperRand) GetBeaconRandomness(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return r.ChainGetRandomnessFromBeacon(ctx, pers, round, entropy)
}
