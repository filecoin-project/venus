package types

import cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

func init() {
	cbor.RegisterCborType(Commitments{})
}

// CommitmentLength is the length of a single commitment (in bytes).
const CommitmentLength = 32

// Commitments is a struct containing the replica and data commitments produced
// when sealing a sector.
type Commitments struct {
	CommD     [CommitmentLength]byte
	CommR     [CommitmentLength]byte
	CommRStar [CommitmentLength]byte
}
