package crypto

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
)

// RandomnessSource provides randomness to actors.
type RandomnessSource interface {
	Randomness(tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

type RandomSeed []byte

type ChainSampler interface {
	Sample(epoch abi.ChainEpoch) (RandomSeed, error)
}

// A randomness source that seeds computations with a sample drawn from a chain epoch.
type ChainRandomnessSource struct {
	Sampler ChainSampler
}

func (c *ChainRandomnessSource) Randomness(tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	seed, err := c.Sampler.Sample(epoch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sample chain for randomness")
	}
	return blendEntropy(tag, seed, entropy)
}

// A randomness source for use when computing genesis state (the state that the genesis block points to as parent state).
// There is no chain to sample a seed from.
type GenesisRandomnessSource struct{}

func (g *GenesisRandomnessSource) Randomness(tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	if epoch > 0 {
		return nil, fmt.Errorf("invalid use of genesis randomness source for epoch %d", epoch)
	}
	seed := []byte{}
	return blendEntropy(tag, seed, entropy)
}

func blendEntropy(tag crypto.DomainSeparationTag, seed RandomSeed, entropy []byte) (abi.Randomness, error) {
	buffer := bytes.Buffer{}
	err := binary.Write(&buffer, binary.BigEndian, int64(tag))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write tag for randomness")
	}
	_, err = buffer.Write(seed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write seed for randomness")
	}
	_, err = buffer.Write(entropy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write entropy for randomness")
	}
	bufHash := blake2b.Sum256(buffer.Bytes())
	return bufHash[:], nil
}
