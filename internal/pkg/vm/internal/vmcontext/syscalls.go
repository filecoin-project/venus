package vmcontext

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specscrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"
)

type syscalls struct {
	gasTank   *GasTracker
	pricelist gascost.Pricelist
}

var _ specsruntime.Syscalls = (*syscalls)(nil)

// VerifySignature implements Syscalls.
func (sys syscalls) VerifySignature(signature specscrypto.Signature, signer address.Address, plaintext []byte) error {
	// Dragons: this lets all id addresses off the hook -- we need to remove this
	// once market actor code actually checks proposal signature.  Depending on how
	// that works we may want to do id address to pubkey address lookup here or we
	// might defer that to VM
	if signer.Protocol() == address.ID {
		return nil
	}
	sys.gasTank.Charge(sys.pricelist.OnVerifySignature(signature.Type, len(plaintext)))
	return crypto.ValidateSignature(plaintext, signer, signature)
}

// HashBlake2b implements Syscalls.
func (sys syscalls) HashBlake2b(data []byte) [32]byte {
	sys.gasTank.Charge(sys.pricelist.OnHashing(len(data)))
	return blake2b.Sum256(data)
}

// ComputeUnsealedSectorCID implements Syscalls.
// Review: why is this returning an error instead of aborting? is this failing recoverable by actors?
func (sys syscalls) ComputeUnsealedSectorCID(proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	sys.gasTank.Charge(sys.pricelist.OnComputeUnsealedSectorCid(proof, &pieces))
	panic("TODO")
}

// VerifySeal implements Syscalls.
func (sys syscalls) VerifySeal(info abi.SealVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifySeal(info))
	panic("TODO")
}

// VerifyPoSt implements Syscalls.
func (sys syscalls) VerifyPoSt(info abi.PoStVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyPost(info))
	panic("TODO")
}

// VerifyConsensusFault implements Syscalls.
func (sys syscalls) VerifyConsensusFault(h1, h2 []byte) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyConsensusFault())
	panic("TODO")
}
