package proofs

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// IsPoStValidWithVerifier is a wrapper for running VerifyPoSt.
// It creates the VerifyPoSTRequest and wraps errors when received.
//
// This is both to simplify PoSt verification by encapsulating the process,
// and allow for better unit testing by being able to provide a test verifier
// (see FakeVerifier in proofs/testing.go)
//
// returns:
//     bool:  if this proof is valid (i.e. the validation test completed)
//     error:  non-nil if VerifyPoST did not complete its checking
// params:
//   verifier:      the verifier to used to verify the proof
//   commRs:  	    the replica commitments that pertain to the provided proof
//   challengeSeed: the challenge seed used when creating the proof
//   faults: 	    any faults produced when creating the proof
//   proof:   		the proof to test
//   challengeSeed:  the challenge seed used when creating the proof
func IsPoStValidWithVerifier(verifier Verifier, commRs []CommR, challengeSeed PoStChallengeSeed, faults []uint64, proof PoStProof) (bool, error) {
	req := VerifyPoSTRequest{
		ChallengeSeed: challengeSeed,
		CommRs:        commRs,
		Faults:        faults,
		Proof:         proof,
	}

	res, err := verifier.VerifyPoST(req)
	if err != nil {
		return false, errors.Wrap(err, "failed to verify PoSt")
	}
	if !res.IsValid {
		return false, nil
	}
	return true, nil
}
