package verification

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// PoStVerifier provides an interface to the proving subsystem.
type PoStVerifier interface {
	VerifyPoSt(info abi.PoStVerifyInfo) (bool, error)
}

// SealVerifier provides an interface to the proving subsystem.
type SealVerifier interface {
	VerifySeal(info abi.SealVerifyInfo) (bool, error)
}

// Verifier verifies PoSt and Seal
type Verifier interface {
	PoStVerifier
	SealVerifier
}
