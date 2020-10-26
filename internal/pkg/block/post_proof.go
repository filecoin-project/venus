package block

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
)

type PoStProof struct {
	_          struct{} `cbor:",toarray"`
	PoStProof  abi.RegisteredPoStProof
	ProofBytes []byte
}

func FromAbiProofArr(abiProof []proof.PoStProof) []PoStProof {
	pof := make([]PoStProof, len(abiProof))
	for index, postProof := range abiProof {
		pof[index] = PoStProof{
			PoStProof:  postProof.PoStProof,
			ProofBytes: postProof.ProofBytes,
		}
	}
	return pof
}

func FromAbiProof(abiProof proof.PoStProof) PoStProof {
	return PoStProof{
		PoStProof:  abiProof.PoStProof,
		ProofBytes: abiProof.ProofBytes,
	}
}

func (p PoStProof) AsAbiProof() proof.PoStProof {
	return proof.PoStProof{
		PoStProof:  p.PoStProof,
		ProofBytes: p.ProofBytes,
	}
}
