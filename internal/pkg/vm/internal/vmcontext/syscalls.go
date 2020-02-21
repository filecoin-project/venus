package vmcontext

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specscrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"
)

type syscalls struct {
	gasTank *GasTracker
}

var _ specsruntime.Syscalls = (*syscalls)(nil)

// VerifySignature implements Syscalls.
func (sys syscalls) VerifySignature(signature specscrypto.Signature, signer address.Address, plaintext []byte) bool {
	return crypto.IsValidSignature(plaintext, signer, signature)
}

// HashBlake2b implements Syscalls.
func (sys syscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

// ComputeUnsealedSectorCID implements Syscalls.
// Review: why is this returning an error instead of aborting? is this failing recoverable by actors?
func (sys syscalls) ComputeUnsealedSectorCID(sectorSize abi.SectorSize, pieces []abi.PieceInfo) (cid.Cid, error) {
	panic("TODO")
}

// VerifySeal implements Syscalls.
func (sys syscalls) VerifySeal(sectorSize abi.SectorSize, info abi.SealVerifyInfo) bool {
	panic("TODO")
}

// VerifyPoSt implements Syscalls.
func (sys syscalls) VerifyPoSt(sectorSize abi.SectorSize, info abi.PoStVerifyInfo) bool {
	panic("TODO")
}

// VerifyConsensusFault implements Syscalls.
func (sys syscalls) VerifyConsensusFault(h1, h2 []byte) bool {
	panic("TODO")
}
