package chain

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

// A sampler draws randomness seeds from the chain.
type Sampler struct {
	reader TipSetProvider
}

func NewSampler(reader TipSetProvider) *Sampler {
	return &Sampler{reader}
}

// Draws a randomness seed from the chain identified by `head` and the highest tipset with height <= `epoch`.
// If `head` is empty (as when processing the genesis block), the seed is empty.
func (s *Sampler) Sample(ctx context.Context, head block.TipSetKey, epoch abi.ChainEpoch) (crypto.RandomSeed, error) {
	var ticket block.Ticket
	if !head.Empty() {
		start, err := s.reader.GetTipSet(head)
		if err != nil {
			return nil, err
		}
		// Note: it is not an error to have epoch > start.Height(); in the case of a run of null blocks the
		// sought-after height may be after the base (last non-empty) tipset.
		// It's also not an error for the requested epoch to be negative.

		tip, err := s.findTipsetAtEpoch(ctx, start, epoch)
		if err != nil {
			return nil, err
		}
		ticket, err = tip.MinTicket()
		if err != nil {
			return nil, err
		}
	} else {
		// Sampling for the genesis block.
		ticket.VRFProof = []byte{}
	}

	buf := bytes.Buffer{}
	buf.Write(ticket.VRFProof)
	err := binary.Write(&buf, binary.BigEndian, epoch)
	if err != nil {
		return nil, err
	}

	bufHash := blake2b.Sum256(buf.Bytes())
	return bufHash[:], err
}

// Finds the the highest tipset with height <= the requested epoch, by traversing backward from start.
func (s *Sampler) findTipsetAtEpoch(ctx context.Context, start block.TipSet, epoch abi.ChainEpoch) (ts block.TipSet, err error) {
	iterator := IterAncestors(ctx, s.reader, start)
	var h abi.ChainEpoch
	for ; !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return
		}
		ts = iterator.Value()
		h, err = ts.Height()
		if err != nil {
			return
		}
		if h <= epoch {
			break
		}
	}
	// If the iterator completed, ts is the genesis tipset.
	return
}
