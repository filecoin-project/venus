package consensus

import (
	"context"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"

	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/types"
)

//ChainRandomness define randomness method in filecoin
type ChainRandomness interface {
	ChainGetRandomnessFromBeacon(ctx context.Context, key types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromTickets(ctx context.Context, tsk types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
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
	return h.chain.ChainGetRandomnessFromBeacon(ctx, h.head, personalization, randEpoch, entropy)
}

func (h HeadRandomness) ChainGetRandomnessFromTickets(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.ChainGetRandomnessFromTickets(ctx, h.head, personalization, randEpoch, entropy)
}
