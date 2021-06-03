package consensus

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/types"
)

type ChainRandomness interface {
	GetChainRandomness(ctx context.Context, tsk types.TipSetKey, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte, lookback bool) ([]byte, error)
	GetBeaconRandomness(ctx context.Context, tsk types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, lookback bool) (abi.Randomness, error)
}

// A Chain randomness source with a fixed Head tipset key.
type HeadRandomness struct {
	Chain ChainRandomness
	Head  types.TipSetKey
}

func (h *HeadRandomness) GetChainRandomnessLookingBack(ctx context.Context, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return h.Chain.GetChainRandomness(ctx, h.Head, tag, epoch, entropy, true)
}

func (h *HeadRandomness) GetChainRandomnessLookingForward(ctx context.Context, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return h.Chain.GetChainRandomness(ctx, h.Head, tag, epoch, entropy, false)
}

func (h *HeadRandomness) GetBeaconRandomnessLookingBack(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return h.Chain.GetBeaconRandomness(ctx, h.Head, pers, round, entropy, true)
}

func (h *HeadRandomness) GetBeaconRandomnessLookingForward(ctx context.Context, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return h.Chain.GetBeaconRandomness(ctx, h.Head, pers, round, entropy, false)

}
