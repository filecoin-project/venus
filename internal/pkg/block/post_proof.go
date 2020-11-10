package block

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
)

type PoStProof struct {
	_          struct{} `cbor:",toarray"`
	PoStProof  abi.RegisteredPoStProof
	ProofBytes []byte
}

func FromAbiProofArr(abiProof []builtin.PoStProof) []PoStProof {
	pof := make([]PoStProof, len(abiProof))
	for index, postProof := range abiProof {
		pof[index] = PoStProof{
			PoStProof:  postProof.PoStProof,
			ProofBytes: postProof.ProofBytes,
		}
	}
	return pof
}

func FromAbiProof(abiProof builtin.PoStProof) PoStProof {
	return PoStProof{
		PoStProof:  abiProof.PoStProof,
		ProofBytes: abiProof.ProofBytes,
	}
}

func (p PoStProof) AsAbiProof() builtin.PoStProof {
	return builtin.PoStProof{
		PoStProof:  p.PoStProof,
		ProofBytes: p.ProofBytes,
	}
}
