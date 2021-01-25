package consensus

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/block"
)

type ChainRandomness interface {
	SampleChainRandomness(ctx context.Context, head block.TipSetKey, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

// A Chain randomness source with a fixed Head tipset key.
type HeadRandomness struct {
	Chain ChainRandomness
	Head  block.TipSetKey
}

func (h *HeadRandomness) Randomness(ctx context.Context, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.Chain.SampleChainRandomness(ctx, h.Head, tag, epoch, entropy)
}

func (h *HeadRandomness) GetRandomnessFromBeacon(ctx context.Context, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.Chain.ChainGetRandomnessFromBeacon(ctx, h.Head, tag, epoch, entropy)
}
