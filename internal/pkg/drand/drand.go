package drand

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/abi"
)

type Schedule []BeaconPoint

func (bs Schedule) BeaconForEpoch(e abi.ChainEpoch) IFace {
	for i := len(bs) - 1; i >= 0; i-- {
		bp := bs[i]
		if e >= bp.Start {
			return bp.Beacon
		}
	}
	return bs[0].Beacon
}

type BeaconPoint struct {
	Start  abi.ChainEpoch
	Beacon IFace
}

// IFace represents a system that provides randomness to Lotus.
// Other components interrogate the RandomBeacon to acquire randomness that's
// valid for a specific chain epoch. Also to verify beacon entries that have
// been posted on chain.
type IFace interface {
	ReadEntry(ctx context.Context, drandRound Round) (*Entry, error)
	VerifyEntry(parent, child *Entry) (bool, error)
	MaxBeaconRoundForEpoch(abi.ChainEpoch) Round
}

// Round is a type for recording drand round indexes
type Round uint64

// A verifiable entry from a beacon chain, carrying round and randomness information.
type Entry struct {
	_     struct{} `cbor:",toarray"`
	Round Round
	Data  []byte
}

func ValidateBlockValues(bSchedule Schedule, h *block.Block, parentEpoch abi.ChainEpoch, prevEntry Entry) error {
	{
		parentBeacon := bSchedule.BeaconForEpoch(parentEpoch)
		currBeacon := bSchedule.BeaconForEpoch(h.Height)
		if parentBeacon != currBeacon {
			if len(h.BeaconEntries) != 2 {
				return xerrors.Errorf("expected two beacon entries at beacon fork, got %d", len(h.BeaconEntries))
			}
			_,err := currBeacon.VerifyEntry(h.BeaconEntries[1], h.BeaconEntries[0])
			if err != nil {
				return xerrors.Errorf("beacon at fork point invalid: (%v, %v): %w",
					h.BeaconEntries[1], h.BeaconEntries[0], err)
			}
			return nil
		}
	}

	// TODO: fork logic
	b := bSchedule.BeaconForEpoch(h.Height)
	maxRound := b.MaxBeaconRoundForEpoch(h.Height)
	if maxRound == prevEntry.Round {
		if len(h.BeaconEntries) != 0 {
			return xerrors.Errorf("expected not to have any beacon entries in this block, got %d", len(h.BeaconEntries))
		}
		return nil
	}

	if len(h.BeaconEntries) == 0 {
		return xerrors.Errorf("expected to have beacon entries in this block, but didn't find any")
	}

	last := h.BeaconEntries[len(h.BeaconEntries)-1]
	if last.Round != maxRound {
		return xerrors.Errorf("expected final beacon entry in block to be at round %d, got %d", maxRound, last.Round)
	}

	for i, e := range h.BeaconEntries {
		if _,err := b.VerifyEntry(e, &prevEntry); err != nil {
			return xerrors.Errorf("beacon entry %d (%d - %x (%d)) was invalid: %w", i, e.Round, e.Data, len(e.Data), err)
		}
		prevEntry = *e
	}

	return nil
}
