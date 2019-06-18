package storagedeal

import (
	"fmt"
)

// State signifies the state of a deal
type State int

const (
	// Unset indicates a programmer error and should never appear in an actual message.
	Unset = State(iota)

	// Unknown signifies an unknown negotiation
	Unknown

	// Rejected means the deal was rejected for some reason
	Rejected

	// Accepted means the deal was accepted but hasnt yet started
	Accepted

	// Started means the deal has started and the transfer is in progress
	Started

	// Failed means the deal has failed for some reason
	Failed

	// Staged means that data has been received and staged into a sector, but is not sealed yet.
	Staged

	// Complete means that the sector that the deal is contained in has been sealed and its commitment posted on chain.
	Complete
)

func (s State) String() string {
	switch s {
	case Unset:
		return "unset"
	case Unknown:
		return "unknown"
	case Rejected:
		return "rejected"
	case Accepted:
		return "accepted"
	case Started:
		return "started"
	case Failed:
		return "failed"
	case Staged:
		return "staged"
	case Complete:
		return "complete"
	default:
		return fmt.Sprintf("<unrecognized %d>", s)
	}
}
