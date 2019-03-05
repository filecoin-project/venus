package storagedeal

import (
	"fmt"
)

// State signifies the state of a deal
type State int

const (
	// Unknown signifies an unknown negotiation
	Unknown = State(iota)

	// Rejected means the deal was rejected for some reason
	Rejected

	// Accepted means the deal was accepted but hasnt yet started
	Accepted

	// Started means the deal has started and the transfer is in progress
	Started

	// Failed means the deal has failed for some reason
	Failed

	// Posted means the deal has been posted to the blockchain
	Posted

	// Complete means the deal is complete
	// TODO: distinguish this from 'Posted'
	Complete

	// Staged means that the data in the deal has been staged into a sector
	Staged
)

func (s State) String() string {
	switch s {
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
	case Posted:
		return "posted"
	case Complete:
		return "complete"
	case Staged:
		return "staged"
	default:
		return fmt.Sprintf("<unrecognized %d>", s)
	}
}
