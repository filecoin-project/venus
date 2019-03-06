package proofs

// PoStBytesLen is the length of the Proof of SpaceTime proof.
const PoStBytesLen uint = 192

// SealBytesLen is the length of the proof of Seal Proof of Replication.
const SealBytesLen uint = 384

// PoStChallengeSeedBytesLen is the number of bytes in the Proof of SpaceTime challenge seed.
const PoStChallengeSeedBytesLen uint = 32

// CommitmentBytesLen is the number of bytes in a CommR, CommD, and CommRStar.
const CommitmentBytesLen uint = 32

// PoStProof is the byte representation of the Proof of SpaceTime proof
type PoStProof [PoStBytesLen]byte

// SealProof is the byte representation of the Seal Proof of Replication
type SealProof [SealBytesLen]byte

// PoStChallengeSeed is an input to the proof-of-spacetime generation and verification methods.
type PoStChallengeSeed [PoStChallengeSeedBytesLen]byte

// CommR is the merkle root of the replicated data. It is an output of the
// sector sealing (PoRep) process.
type CommR [CommitmentBytesLen]byte

// CommD is the merkle root of the original user data. It is an output of the
// sector sealing (PoRep) process.
type CommD [CommitmentBytesLen]byte

// CommRStar is a hash of intermediate layers. It is an output of the sector
// sealing (PoRep) process.
type CommRStar [CommitmentBytesLen]byte
