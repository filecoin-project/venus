package chain

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
)

type RandomSeed []byte

///// Chain sampling /////

type ChainSampler interface { //nolint
	Sample(ctx context.Context, epoch abi.ChainEpoch, lookback bool) (RandomSeed, error)
	GetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, lookback bool) (abi.Randomness, error)
}

// A sampler for use when computing genesis state (the state that the genesis block points to as parent state).
// There is no chain to sample a seed from.
type GenesisSampler struct {
	VRFProof types.VRFPi
}

func (g *GenesisSampler) Sample(_ context.Context, epoch abi.ChainEpoch, _ bool) (RandomSeed, error) {
	if epoch > 0 {
		return nil, fmt.Errorf("invalid use of genesis sampler for epoch %d", epoch)
	}
	return MakeRandomSeed(g.VRFProof)
}

func (g *GenesisSampler) GetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte, _ bool) (abi.Randomness, error) {
	//use trust beacon value todo
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return out, nil
}

// Computes a random seed from raw ticket bytes.
// A randomness seed is the VRF digest of the minimum ticket of the tipset at or before the requested epoch
func MakeRandomSeed(rawVRFProof types.VRFPi) (RandomSeed, error) {
	digest := rawVRFProof.Digest()
	return digest[:], nil
}

///// GetRandomnessFromTickets derivation /////

// RandomnessSource provides randomness to actors.
type RandomnessSource interface {
	GetChainRandomnessLookingBack(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetChainRandomnessLookingForward(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomnessLookingBack(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomnessLookingForward(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
}

// A randomness source that seeds computations with a sample drawn from a chain epoch.
type ChainRandomnessSource struct { //nolint
	Sampler ChainSampler
}

func (c *ChainRandomnessSource) GetChainRandomnessLookingBack(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	seed, err := c.Sampler.Sample(ctx, round, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sample chain for randomness")
	}
	return BlendEntropy(pers, seed, round, entropy)
}

func (c *ChainRandomnessSource) GetChainRandomnessLookingForward(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	seed, err := c.Sampler.Sample(ctx, round, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sample chain for randomness")
	}
	return BlendEntropy(pers, seed, round, entropy)
}

func (c *ChainRandomnessSource) GetBeaconRandomnessLookingBack(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return c.Sampler.GetRandomnessFromBeacon(ctx, pers, round, entropy, true)
}

func (c *ChainRandomnessSource) GetBeaconRandomnessLookingForward(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return c.Sampler.GetRandomnessFromBeacon(ctx, pers, round, entropy, false)
}

func (c *ChainRandomnessSource) GetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return c.Sampler.GetRandomnessFromBeacon(ctx, personalization, randEpoch, entropy, true)
}

func BlendEntropy(tag crypto.DomainSeparationTag, seed RandomSeed, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	buffer := bytes.Buffer{}
	err := binary.Write(&buffer, binary.BigEndian, int64(tag))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write tag for randomness")
	}
	_, err = buffer.Write(seed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write seed for randomness")
	}
	err = binary.Write(&buffer, binary.BigEndian, int64(epoch))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write epoch for randomness")
	}
	_, err = buffer.Write(entropy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write entropy for randomness")
	}
	bufHash := blake2b.Sum256(buffer.Bytes())
	return bufHash[:], nil
}
