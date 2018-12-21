package proofs

import (
	"fmt"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// IsPoStValidWithProver is a wrapper for running VerifyPoSt.
// It creates the VerifyPoSTRequest and wraps errors when received.
//
// This is both to simplify PoSt verification by encapsulating the process,
// and allow for better unit testing by being able to provide a test prover
// (see FakeProver in proofs/testing.go)
//
// returns:
//     bool:  if this proof is valid (i.e. the validation test completed)
//     error:  non-nil if the length is wrong or if the VerifyPoST did not complete its checking
// params:
//   prover:  the Prover to use for testing the Proof
//   proof:   the proof to test
//   challenge:  the challenge used for creating the block
func IsPoStValidWithProver(prover Prover, proof []byte, challenge []byte) (bool, error) {
	if uint(len(proof)) != SnarkBytesLen {
		return false, fmt.Errorf("proof must be %d bytes, but is %d bytes", SnarkBytesLen, len(proof))
	}

	req := VerifyPoSTRequest{
		Challenge: challenge,
		Proof:     PoStProof{},
	}

	copy(req.Proof[:], proof[:])
	res, err := prover.VerifyPoST(req)
	if err != nil {
		return false, errors.Wrap(err, "failed to verify PoSt")
	}
	if !res.IsValid {
		return false, nil
	}
	return true, nil
}
