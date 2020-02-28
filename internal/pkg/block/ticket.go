package block

import (
	"fmt"

	"github.com/minio/blake2b-simd"
)

// A Ticket is a marker of a tick of the blockchain's clock.  It is the source
// of randomness for proofs of storage and leader election.  It is generated
// by the miner of a block using a VRF.
type Ticket struct {
	_ struct{} `cbor:",toarray"`
	// A proof output by running a VRF on the VRFProof of the parent ticket
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

// Digest returns the digest (hash) of a proof, for use generating challenges etc.
func (p VRFPi) Digest() [32]byte {
	proofDigest := blake2b.Sum256(p)
	return proofDigest
}
