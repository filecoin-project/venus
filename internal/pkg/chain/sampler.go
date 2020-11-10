package chain

import (
	"context"
	"encoding/binary"

	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/minio/blake2b-simd"
	xerrors "github.com/pkg/errors"
)

type TipSetByHeight interface {
	GetTipSet(key block.TipSetKey) (*block.TipSet, error)
	GetTipSetByHeight(ctx context.Context, ts *block.TipSet, h abi.ChainEpoch, prev bool) (*block.TipSet, error)
}

// Creates a new sampler for the chain identified by `head`.
func NewRandomnessSamplerAtTipSet(reader TipSetByHeight, genesisTicket block.Ticket, head block.TipSetKey) *RandomnessSamplerAtTipSet {
	return &RandomnessSamplerAtTipSet{
		sampler: NewSampler(reader, genesisTicket),
		head:    head,
	}
}

// A sampler draws randomness seeds from the chain.
//
// This implementation doesn't do any caching: it traverses the chain each time. A cache that could be directly
// indexed by epoch could speed up repeated samples from the same chain.
type Sampler struct {
	reader        TipSetByHeight
	genesisTicket block.Ticket
}

func NewSampler(reader TipSetByHeight, genesisTicket block.Ticket) *Sampler {
	return &Sampler{reader, genesisTicket}
}

// Draws a ticket from the chain identified by `head` and the highest tipset with height <= `epoch`.
// If `head` is empty (as when processing the pre-genesis state or the genesis block), the seed derived from
// a fixed genesis ticket.
// Note that this may produce the same value for different, neighbouring epochs when the epoch references a round
// in which no blocks were produced (an empty tipset or "null block"). A caller desiring a unique see for each epoch
// should blend in some distinguishing value (such as the epoch itself) into a hash of this ticket.
func (s *Sampler) SampleTicket(ctx context.Context, head block.TipSetKey, epoch abi.ChainEpoch) (block.Ticket, error) {
	var ticket block.Ticket
	if !head.Empty() {
		start, err := s.reader.GetTipSet(head)
		if err != nil {
			return block.Ticket{}, err
		}

		if epoch > start.EnsureHeight() {
			return block.Ticket{}, xerrors.Errorf("cannot draw randomness from the future")
		}

		searchHeight := epoch
		if searchHeight < 0 {
			searchHeight = 0
		}

		// Note: it is not an error to have epoch > start.Height(); in the case of a run of null blocks the
		// sought-after height may be after the base (last non-empty) tipset.
		// It's also not an error for the requested epoch to be negative.
		tip, err := s.reader.GetTipSetByHeight(ctx, start, searchHeight, true)
		if err != nil {
			return block.Ticket{}, err
		}
		ticket, err = tip.MinTicket()
		if err != nil {
			return block.Ticket{}, err
		}
	} else {
		// Sampling for the genesis state or genesis tipset.
		ticket = s.genesisTicket
	}

	return ticket, nil
}

func (s *Sampler) SampleRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	ts, err := s.reader.GetTipSet(tsk)
	if err != nil {
		return nil, err
	}

	if randEpoch > ts.EnsureHeight() {
		return nil, xerrors.Errorf("cannot draw randomness from the future")
	}

	searchHeight := randEpoch
	if searchHeight < 0 {
		searchHeight = 0
	}

	randTs, err := s.reader.GetTipSetByHeight(ctx, ts, searchHeight, true)
	if err != nil {
		return nil, err
	}

	be, err := FindLatestDRAND(ctx, randTs, s.reader)
	if err != nil {
		return nil, err
	}

	// if at (or just past -- for null epochs) appropriate epoch
	// or at genesis (works for negative epochs)
	return DrawRandomness(be.Data, personalization, randEpoch, entropy)
}

func DrawRandomness(rbase []byte, pers acrypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
	h := blake2b.New256()
	if err := binary.Write(h, binary.BigEndian, int64(pers)); err != nil {
		return nil, xerrors.Errorf("deriving randomness: %s", err)
	}
	VRFDigest := blake2b.Sum256(rbase)
	_, err := h.Write(VRFDigest[:])
	if err != nil {
		return nil, xerrors.Errorf("hashing VRFDigest: %s", err)
	}
	if err := binary.Write(h, binary.BigEndian, round); err != nil {
		return nil, xerrors.Errorf("deriving randomness: %s", err)
	}
	_, err = h.Write(entropy)
	if err != nil {
		return nil, xerrors.Errorf("hashing entropy: %s", err)
	}

	return h.Sum(nil), nil
}

///// A chain sampler with a specific head tipset key. /////

type RandomnessSamplerAtTipSet struct {
	sampler *Sampler
	head    block.TipSetKey
}

func (s *RandomnessSamplerAtTipSet) Sample(ctx context.Context, epoch abi.ChainEpoch) (crypto.RandomSeed, error) {
	ticket, err := s.sampler.SampleTicket(ctx, s.head, epoch)
	if err != nil {
		return nil, err
	}
	return crypto.MakeRandomSeed(ticket.VRFProof)
}

func (s *RandomnessSamplerAtTipSet) GetRandomnessFromBeacon(ctx context.Context, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return s.sampler.SampleRandomnessFromBeacon(ctx, s.head, personalization, randEpoch, entropy)
}
