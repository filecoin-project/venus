package types

import (
	cbor "github.com/ipfs/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(Ticket{})
}

// A Ticket is a marker of a tick of the blockchain's clock.  It is the source
// of randomness for proofs of storage and leader election.  It is generated
// by the miner of a block using a VRF and a VDF.
type Ticket struct {
	// A proof output by running a VRF on the VDFResult of the parent ticket
	VRFProof VRFPi

	// Data derived by running a VDF on VRFProof
	VDFResult VDFY

	// A proof of delay during computation of VDFResult
	VDFProof VDFPi
}

// SortKey returns the canonical byte ordering of the ticket
func (t Ticket) SortKey() []byte {
	return t.VRFProof
}

// VRFPi is the proof output from running a VRF.
type VRFPi []byte

// VDFPi is proof that a VDF operation was applied on input X to get output Y.
type VDFPi []byte

// VDFY is the output of running a VDF operation on some input X.
type VDFY []byte
