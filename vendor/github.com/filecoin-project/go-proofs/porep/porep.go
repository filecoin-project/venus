package porep

import "github.com/filecoin-project/go-proofs/proofs"

// SecurityParameter TODO
type SecurityParameter int

// Time TODO
type Time int

// ProverID TODO
type ProverID []byte

// Challenge TODO
type Challenge int

// Commitment TODO
type Commitment []byte

// Replica TODO
type Replica []byte

// Proof is unspecialized.
type Proof proofs.Proof

// Tau TODO
type Tau struct {
	CommR Commitment
	CommD Commitment
}

// SetupParams are the general porep SetupParams.
type SetupParams struct {
	Lambda SecurityParameter
	Time   Time
}

// PublicParams are the general porep PublicParams
type PublicParams struct {
	Lambda SecurityParameter
	Time   Time
}

// PublicInputs are the general porep PublicInputs
type PublicInputs struct {
	id  ProverID
	r   Challenge
	tau *Tau
}

// PrivateInputs are the general porep PrivateInputs
type PrivateInputs struct {
	replica []byte
}

// PoRep is an interface which must be implemented by all concrete porep implementations.
type PoRep interface {
	// TODO: setup might not be required by all proofs
	Replicate(pp *PublicParams, id ProverID, data []byte) (replica Replica, tau *Tau)
	ExtractAll(pp *PublicParams, id ProverID, replica Replica) (data []byte)
	Extract(pp *PublicParams, id ProverID, replica Replica, nodeID int) []byte
}
