package block

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// PoStProof is a winning post proof included in a block header
type PoStProof struct {
	_               struct{} `cbor:",toarray"`
	RegisteredProof abi.RegisteredProof
	ProofBytes      []byte
}

// NewPoStProof constructs an epost proof from registered proof and bytes
func NewPoStProof(rpp abi.RegisteredProof, bs []byte) PoStProof {
	return PoStProof{
		RegisteredProof: rpp,
		ProofBytes:      bs,
	}
}

// FromABIPoStProofs converts the abi post proof type to a local type for
// serialization purposes
func FromABIPoStProofs(postProofs ...abi.PoStProof) []PoStProof {
	out := make([]PoStProof, len(postProofs))
	for i, p := range postProofs {
		out[i] = PoStProof{RegisteredProof: p.RegisteredProof, ProofBytes: p.ProofBytes}
	}

	return out
}
