package chain

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
)

type RandomSeed []byte

var _ RandomnessSource = (*GenesisRandomnessSource)(nil)

// A sampler for use when computing genesis state (the state that the genesis block points to as parent state).
// There is no chain to sample a seed from.
type GenesisRandomnessSource struct {
	vrf types.VRFPi
}

func NewGenesisRandomnessSource(vrf types.VRFPi) *GenesisRandomnessSource {
	return &GenesisRandomnessSource{vrf: vrf}
}

func (g *GenesisRandomnessSource) ChainGetRandomnessFromBeacon(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return out, nil
}

func (g *GenesisRandomnessSource) ChainGetRandomnessFromTickets(ctx context.Context, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return out, nil
}

func (g *GenesisRandomnessSource) GetChainRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch * 1000))).Read(out) //nolint
	return out, nil
}

func (g *GenesisRandomnessSource) GetChainRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch * 1000))).Read(out) //nolint
	return out, nil
}

func (g *GenesisRandomnessSource) GetBeaconRandomnessV3(ctx context.Context, pers crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return out, nil
}

func (g *GenesisRandomnessSource) GetBeaconRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return out, nil
}

func (g *GenesisRandomnessSource) GetBeaconRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
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
	GetChainRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetChainRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error)
	GetBeaconRandomnessV3(ctx context.Context, pers crypto.DomainSeparationTag, filecoinEpoch abi.ChainEpoch, entropy []byte) ([]byte, error)
}

type TipSetByHeight interface {
	GetTipSet(context.Context, types.TipSetKey) (*types.TipSet, error)
	GetTipSetByHeight(context.Context, *types.TipSet, abi.ChainEpoch, bool) (*types.TipSet, error)
}

var _ RandomnessSource = (*ChainRandomnessSource)(nil)

type NetworkVersionGetter func(context.Context, abi.ChainEpoch) network.Version

// A randomness source that seeds computations with a sample drawn from a chain epoch.
type ChainRandomnessSource struct { //nolint
	reader               TipSetByHeight
	head                 types.TipSetKey
	beacon               beacon.Schedule
	networkVersionGetter NetworkVersionGetter
}

func NewChainRandomnessSource(reader TipSetByHeight, head types.TipSetKey, beacon beacon.Schedule, networkVersionGetter NetworkVersionGetter) RandomnessSource {
	return &ChainRandomnessSource{reader: reader, head: head, beacon: beacon, networkVersionGetter: networkVersionGetter}
}

func (c *ChainRandomnessSource) GetBeaconRandomnessTipset(ctx context.Context, randEpoch abi.ChainEpoch, lookback bool) (*types.TipSet, error) {
	ts, err := c.reader.GetTipSet(ctx, c.head)
	if err != nil {
		return nil, err
	}

	if randEpoch > ts.Height() {
		return nil, fmt.Errorf("cannot draw randomness from the future")
	}

	searchHeight := randEpoch
	if searchHeight < 0 {
		searchHeight = 0
	}

	randTS, err := c.reader.GetTipSetByHeight(ctx, ts, searchHeight, lookback)
	if err != nil {
		return nil, err
	}
	return randTS, nil
}

// Draws a ticket from the chain identified by `head` and the highest tipset with height <= `epoch`.
// If `head` is empty (as when processing the pre-genesis state or the genesis block), the seed derived from
// a fixed genesis ticket.
// Note that this may produce the same value for different, neighbouring epochs when the epoch references a round
// in which no blocks were produced (an empty tipset or "null block"). A caller desiring a unique see for each epoch
// should blend in some distinguishing value (such as the epoch itself) into a hash of this ticket.
func (c *ChainRandomnessSource) GetChainRandomness(ctx context.Context, epoch abi.ChainEpoch, lookback bool) (types.Ticket, error) {
	if !c.head.IsEmpty() {
		start, err := c.reader.GetTipSet(ctx, c.head)
		if err != nil {
			return types.Ticket{}, err
		}

		if epoch > start.Height() {
			return types.Ticket{}, fmt.Errorf("cannot draw randomness from the future")
		}

		searchHeight := epoch
		if searchHeight < 0 {
			searchHeight = 0
		}

		// Note: it is not an error to have epoch > start.Height(); in the case of a run of null blocks the
		// sought-after height may be after the base (last non-empty) tipset.
		// It's also not an error for the requested epoch to be negative.
		tip, err := c.reader.GetTipSetByHeight(ctx, start, searchHeight, lookback)
		if err != nil {
			return types.Ticket{}, err
		}
		return *tip.MinTicket(), nil
	}
	return types.Ticket{}, fmt.Errorf("cannot get ticket for empty tipset")
}

// network v0-12
func (c *ChainRandomnessSource) GetChainRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	ticket, err := c.GetChainRandomness(ctx, round, true)
	if err != nil {
		return nil, err
	}
	// if at (or just past -- for null epochs) appropriate epoch
	// or at genesis (works for negative epochs)
	return DrawRandomness(ticket.VRFProof, pers, round, entropy)
}

// network v13 and on
func (c *ChainRandomnessSource) GetChainRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	ticket, err := c.GetChainRandomness(ctx, round, false)
	if err != nil {
		return nil, err
	}
	// if at (or just past -- for null epochs) appropriate epoch
	// or at genesis (works for negative epochs)
	return DrawRandomness(ticket.VRFProof, pers, round, entropy)
}

// network v0-12
func (c *ChainRandomnessSource) GetBeaconRandomnessV1(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	randTS, err := c.GetBeaconRandomnessTipset(ctx, round, true)
	if err != nil {
		return nil, err
	}

	be, err := FindLatestDRAND(ctx, randTS, c.reader)
	if err != nil {
		return nil, err
	}

	// if at (or just past -- for null epochs) appropriate epoch
	// or at genesis (works for negative epochs)
	return DrawRandomness(be.Data, pers, round, entropy)
}

// network v13
func (c *ChainRandomnessSource) GetBeaconRandomnessV2(ctx context.Context, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	randTS, err := c.GetBeaconRandomnessTipset(ctx, round, false)
	if err != nil {
		return nil, err
	}

	be, err := FindLatestDRAND(ctx, randTS, c.reader)
	if err != nil {
		return nil, err
	}

	// if at (or just past -- for null epochs) appropriate epoch
	// or at genesis (works for negative epochs)
	return DrawRandomness(be.Data, pers, round, entropy)
}

// network v14 and on
func (c *ChainRandomnessSource) GetBeaconRandomnessV3(ctx context.Context, pers crypto.DomainSeparationTag, filecoinEpoch abi.ChainEpoch, entropy []byte) ([]byte, error) {
	if filecoinEpoch < 0 {
		return c.GetBeaconRandomnessV2(ctx, pers, filecoinEpoch, entropy)
	}

	be, err := c.extractBeaconEntryForEpoch(ctx, filecoinEpoch)
	if err != nil {
		log.Errorf("failed to get beacon entry as expected: %s", err)
		return nil, err
	}

	return DrawRandomness(be.Data, pers, filecoinEpoch, entropy)
}

func (c *ChainRandomnessSource) extractBeaconEntryForEpoch(ctx context.Context, filecoinEpoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	randTS, err := c.GetBeaconRandomnessTipset(ctx, filecoinEpoch, false)
	if err != nil {
		return nil, err
	}

	nv := c.networkVersionGetter(ctx, filecoinEpoch)

	round := c.beacon.BeaconForEpoch(filecoinEpoch).MaxBeaconRoundForEpoch(nv, filecoinEpoch)

	for i := 0; i < 20; i++ {
		cbe := randTS.Blocks()[0].BeaconEntries
		for _, v := range cbe {
			if v.Round == round {
				return &v, nil
			}
		}

		next, err := c.reader.GetTipSet(ctx, randTS.Parents())
		if err != nil {
			return nil, fmt.Errorf("failed to load parents when searching back for beacon entry: %w", err)
		}

		randTS = next
	}

	return nil, fmt.Errorf("didn't find beacon for round %d (epoch %d)", round, filecoinEpoch)
}

//BlendEntropy get randomness with chain value. sha256(buf(tag, seed, epoch, entropy))
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

func DrawRandomness(rbase []byte, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	h := blake2b.New256()
	if err := binary.Write(h, binary.BigEndian, int64(pers)); err != nil {
		return nil, fmt.Errorf("deriving randomness: %s", err)
	}
	VRFDigest := blake2b.Sum256(rbase)
	_, err := h.Write(VRFDigest[:])
	if err != nil {
		return nil, fmt.Errorf("hashing VRFDigest: %s", err)
	}
	if err := binary.Write(h, binary.BigEndian, round); err != nil {
		return nil, fmt.Errorf("deriving randomness: %s", err)
	}
	_, err = h.Write(entropy)
	if err != nil {
		return nil, fmt.Errorf("hashing entropy: %s", err)
	}

	return h.Sum(nil), nil
}
