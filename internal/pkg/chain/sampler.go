package chain

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// Creates a new sampler for the chain identified by `head`.
func NewRandomnessSamplerAtHead(reader TipSetProvider, genesisTicket block.Ticket, head block.TipSetKey) *RandomnessSamplerAtHead {
	return &RandomnessSamplerAtHead{
		sampler: NewSampler(reader, genesisTicket),
		head:    head,
	}
}

// A sampler draws randomness seeds from the chain.
//
// This implementation doesn't do any caching: it traverses the chain each time. A cache that could be directly
// indexed by epoch could speed up repeated samples from the same chain.
type Sampler struct {
	reader        TipSetProvider
	genesisTicket block.Ticket
}

func NewSampler(reader TipSetProvider, genesisTicket block.Ticket) *Sampler {
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
		// Note: it is not an error to have epoch > start.Height(); in the case of a run of null blocks the
		// sought-after height may be after the base (last non-empty) tipset.
		// It's also not an error for the requested epoch to be negative.

		tip, err := FindTipsetAtEpoch(ctx, start, epoch, s.reader)
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

///// A chain sampler with a specific head tipset key. /////

type RandomnessSamplerAtHead struct {
	sampler *Sampler
	head    block.TipSetKey
}

func (s *RandomnessSamplerAtHead) Sample(ctx context.Context, epoch abi.ChainEpoch) (crypto.RandomSeed, error) {
	ticket, err := s.sampler.SampleTicket(ctx, s.head, epoch)
	if err != nil {
		return nil, err
	}
	return crypto.MakeRandomSeed(ticket.VRFProof)
}
