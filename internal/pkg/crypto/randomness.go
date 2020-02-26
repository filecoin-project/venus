package crypto

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
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
	TicketBytes []byte
}

func (g *GenesisSampler) Sample(_ context.Context, epoch abi.ChainEpoch) (RandomSeed, error) {
	if epoch > 0 {
		return nil, fmt.Errorf("invalid use of genesis sampler for epoch %d", epoch)
	}
	return MakeRandomSeed(g.TicketBytes, epoch)
}

// Computes a random seed from raw ticket bytes and the epoch from which the ticket was requested
// (which may not match the epoch it actually came from).
func MakeRandomSeed(rawTicket []byte, epoch abi.ChainEpoch) (RandomSeed, error) {
	buf := bytes.Buffer{}
	vrfDigest := blake2b.Sum256(rawTicket)
	buf.Write(vrfDigest[:])
	err := binary.Write(&buf, binary.BigEndian, epoch)
	if err != nil {
		return nil, err
	}

	bufHash := blake2b.Sum256(buf.Bytes())
	return bufHash[:], nil
}

///// Randomness derivation /////

// RandomnessSource provides randomness to actors.
type RandomnessSource interface {
	Randomness(ctx context.Context, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
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
