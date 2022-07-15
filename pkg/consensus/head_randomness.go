package consensus

import (
	"context"

	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
)

// ChainRandomness define randomness method in filecoin
type ChainRandomness interface {
	StateGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error)
	StateGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, tsk types.TipSetKey) (abi.Randomness, error)
}

var _ vmcontext.HeadChainRandomness = (*HeadRandomness)(nil)

// A Chain randomness source with a fixed Head tipset key.
type HeadRandomness struct {
	chain ChainRandomness
	head  types.TipSetKey
}

func NewHeadRandomness(chain ChainRandomness, head types.TipSetKey) *HeadRandomness {
	return &HeadRandomness{chain: chain, head: head}
}

func (h HeadRandomness) ChainGetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.StateGetRandomnessFromBeacon(ctx, personalization, randEpoch, entropy, h.head)
}

func (h HeadRandomness) ChainGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.StateGetRandomnessFromTickets(ctx, personalization, randEpoch, entropy, h.head)
}
