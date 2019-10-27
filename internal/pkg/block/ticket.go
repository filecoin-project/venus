package block

import (
	"fmt"
)

// A Ticket is a marker of a tick of the blockchain's clock.  It is the source
// of randomness for proofs of storage and leader election.  It is generated
// by the miner of a block using a VRF and a VDF.
type Ticket struct {
	// A proof output by running a VRF on the VDFResult of the parent ticket
	VRFProof VRFPi
}

// SortKey returns the canonical byte ordering of the ticket
func (t Ticket) SortKey() []byte {
	return t.VRFProof
}

// String returns the string representation of the VRFProof of the ticket
func (t Ticket) String() string {
	return fmt.Sprintf("%x", t.VRFProof)
}

// VRFPi is the proof output from running a VRF.
type VRFPi []byte
