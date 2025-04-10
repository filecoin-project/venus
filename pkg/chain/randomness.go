package chain

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"

	"github.com/filecoin-project/venus/pkg/beacon"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"
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

func (g *GenesisRandomnessSource) GetBeaconRandomness(ctx context.Context, randEpoch abi.ChainEpoch) ([32]byte, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return *(*[32]byte)(out), nil
}
func (g *GenesisRandomnessSource) GetChainRandomness(ctx context.Context, randEpoch abi.ChainEpoch) ([32]byte, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return *(*[32]byte)(out), nil
}
func (g *GenesisRandomnessSource) GetBeaconEntry(_ context.Context, randEpoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	out := make([]byte, 32)
	_, _ = rand.New(rand.NewSource(int64(randEpoch))).Read(out) //nolint
	return &types.BeaconEntry{Round: 10, Data: out}, nil
}

// Computes a random seed from raw ticket bytes.
// A randomness seed is the VRF digest of the minimum ticket of the tipset at or before the requested epoch
func MakeRandomSeed(rawVRFProof types.VRFPi) (RandomSeed, error) {
	digest := rawVRFProof.Digest()
	return digest[:], nil
}

///// GetRandomnessFromTickets derivation /////

type RandomnessSource = vm.ChainRandomness

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
func (c *ChainRandomnessSource) getChainRandomness(ctx context.Context, epoch abi.ChainEpoch, lookback bool) (types.Ticket, error) {
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
func (c *ChainRandomnessSource) GetChainRandomnessV1(ctx context.Context, round abi.ChainEpoch) ([32]byte, error) {
	ticket, err := c.getChainRandomness(ctx, round, true)
	if err != nil {
		return [32]byte{}, err
	}

	return blake2b.Sum256(ticket.VRFProof), nil
}

// network v13 and on
func (c *ChainRandomnessSource) GetChainRandomnessV2(ctx context.Context, round abi.ChainEpoch, lookback bool) ([32]byte, error) {
	ticket, err := c.getChainRandomness(ctx, round, lookback)
	if err != nil {
		return [32]byte{}, err
	}

	return blake2b.Sum256(ticket.VRFProof), nil
}

func (c *ChainRandomnessSource) GetChainRandomness(ctx context.Context, filecoinEpoch abi.ChainEpoch) ([32]byte, error) {
	nv := c.networkVersionGetter(ctx, filecoinEpoch)

	if nv >= network.Version13 {
		return c.GetChainRandomnessV2(ctx, filecoinEpoch, false)
	}

	return c.GetChainRandomnessV2(ctx, filecoinEpoch, true)
}

func (c *ChainRandomnessSource) GetBeaconRandomness(ctx context.Context, filecoinEpoch abi.ChainEpoch) ([32]byte, error) {
	be, err := c.GetBeaconEntry(ctx, filecoinEpoch)
	if err != nil {
		return [32]byte{}, err
	}
	return blake2b.Sum256(be.Data), nil
}

// network v0-12
func (c *ChainRandomnessSource) getBeaconEntryV1(ctx context.Context, round abi.ChainEpoch) (*types.BeaconEntry, error) {
	randTS, err := c.GetBeaconRandomnessTipset(ctx, round, true)
	if err != nil {
		return nil, err
	}

	return c.getLatestBeaconEntry(ctx, randTS)
}

// network v13
func (c *ChainRandomnessSource) getBeaconEntryV2(ctx context.Context, round abi.ChainEpoch) (*types.BeaconEntry, error) {
	randTS, err := c.GetBeaconRandomnessTipset(ctx, round, false)
	if err != nil {
		return nil, err
	}

	return c.getLatestBeaconEntry(ctx, randTS)
}

// network v14 and on
func (c *ChainRandomnessSource) getBeaconEntryV3(ctx context.Context, filecoinEpoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	if filecoinEpoch < 0 {
		return c.getBeaconEntryV2(ctx, filecoinEpoch)
	}

	randTS, err := c.GetBeaconRandomnessTipset(ctx, filecoinEpoch, false)
	if err != nil {
		return nil, err
	}

	nv := c.networkVersionGetter(ctx, filecoinEpoch)

	round := c.beacon.BeaconForEpoch(filecoinEpoch).MaxBeaconRoundForEpoch(nv, filecoinEpoch)

	// Search back for the beacon entry, in normal operation it should be in randTs but for devnets
	// where the blocktime is faster than the beacon period we may need to search back a bit to find
	// the beacon entry for the requested round.
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

func (c *ChainRandomnessSource) GetBeaconEntry(ctx context.Context, randEpoch abi.ChainEpoch) (*types.BeaconEntry, error) {
	rnv := c.networkVersionGetter(ctx, randEpoch)
	if rnv >= network.Version14 {
		be, err := c.getBeaconEntryV3(ctx, randEpoch)
		if err != nil {
			log.Errorf("failed to get beacon entry as expected: %s", err)
		}
		return be, err
	} else if rnv == network.Version13 {
		return c.getBeaconEntryV2(ctx, randEpoch)
	}

	return c.getBeaconEntryV1(ctx, randEpoch)
}

func (c *ChainRandomnessSource) getLatestBeaconEntry(ctx context.Context, ts *types.TipSet) (*types.BeaconEntry, error) {
	cur := ts

	// Search for a beacon entry, in normal operation one should be in the requested tipset, but for
	// devnets where the blocktime is faster than the beacon period we may need to search back a bit
	// to find a tipset with a beacon entry.
	for i := 0; i < 20; i++ {
		cbe := cur.Blocks()[0].BeaconEntries
		if len(cbe) > 0 {
			return &cbe[len(cbe)-1], nil
		}

		if cur.Height() == 0 {
			return nil, fmt.Errorf("made it back to genesis block without finding beacon entry")
		}

		next, err := c.reader.GetTipSet(ctx, cur.Parents())
		if err != nil {
			return nil, fmt.Errorf("failed to load parents when searching back for latest beacon entry: %w", err)
		}
		cur = next
	}

	if os.Getenv("VENUS_IGNORE_DRAND") == "_yes_" {
		return &types.BeaconEntry{
			Data: []byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
		}, nil
	}

	return nil, fmt.Errorf("found NO beacon entries in the 20 latest tipsets")
}

// BlendEntropy get randomness with chain value. sha256(buf(tag, seed, epoch, entropy))
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

func DrawRandomnessFromBase(rbase []byte, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	return DrawRandomnessFromDigest(blake2b.Sum256(rbase), pers, round, entropy)
}

var DrawRandomnessFromDigest = vm.DrawRandomnessFromDigest
