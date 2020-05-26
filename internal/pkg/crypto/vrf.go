package crypto

import "github.com/minio/blake2b-simd"

// VRFPi is the proof output from running a VRF.
type VRFPi []byte

type ElectionProof struct {
	_ struct{} `cbor:",toarray"`
	// A proof output by running a VRF on the VRFProof of the parent ticket
	VRFProof VRFPi
}

// Digest returns the digest (hash) of a proof, for use generating challenges etc.
func (p VRFPi) Digest() [32]byte {
	proofDigest := blake2b.Sum256(p)
	return proofDigest
}
