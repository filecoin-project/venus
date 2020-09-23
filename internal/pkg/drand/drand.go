package drand

import (
	"context"

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
