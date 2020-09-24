package crypto

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
)

type RandomSeed []byte

///// Chain sampling /////

type ChainSampler interface {
	Sample(ctx context.Context, epoch abi.ChainEpoch) (RandomSeed, error)
}

// A sampler for use when computing genesis state (the state that the genesis block points to as parent state).
// There is no chain to sample a seed from.
type GenesisSampler struct {
	VRFProof VRFPi
}

func (g *GenesisSampler) Sample(_ context.Context, epoch abi.ChainEpoch) (RandomSeed, error) {
	if epoch > 0 {
		return nil, fmt.Errorf("invalid use of genesis sampler for epoch %d", epoch)
	}
	return MakeRandomSeed(g.VRFProof)
}

// Computes a random seed from raw ticket bytes.
// A randomness seed is the VRF digest of the minimum ticket of the tipset at or before the requested epoch
func MakeRandomSeed(rawVRFProof VRFPi) (RandomSeed, error) {
	digest := rawVRFProof.Digest()
	return digest[:], nil
}

///// Randomness derivation /////

// RandomnessSource provides randomness to actors.
type RandomnessSource interface {
	Randomness(ctx context.Context, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	GetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

// A randomness source that seeds computations with a sample drawn from a chain epoch.
type ChainRandomnessSource struct {
	Sampler ChainSampler
}

func (c *ChainRandomnessSource) Randomness(ctx context.Context, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	seed, err := c.Sampler.Sample(ctx, epoch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sample chain for randomness")
	}
	return BlendEntropy(tag, seed, epoch, entropy)
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
