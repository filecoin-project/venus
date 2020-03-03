package vmsupport

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

type Syscalls struct {
	// Dependencies on chain state and proofs coming here soon.
}

func NewSyscalls() *Syscalls {
	return &Syscalls{}
}

func (s Syscalls) VerifySignature(epoch abi.ChainEpoch, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	// Dragons: this lets all id addresses off the hook -- we need to remove this
	// once market actor code actually checks proposal signature.  Depending on how
	// that works we may want to do id address to pubkey address lookup here or we
	// might defer that to VM
	if signer.Protocol() == address.ID {
		return nil
	}
	return crypto.ValidateSignature(plaintext, signer, signature)
}

func (s Syscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

func (s Syscalls) ComputeUnsealedSectorCID(proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	panic("implement me")
}

func (s Syscalls) VerifySeal(epoch abi.ChainEpoch, info abi.SealVerifyInfo) error {
	panic("implement me")
}

func (s Syscalls) VerifyPoSt(epoch abi.ChainEpoch, info abi.PoStVerifyInfo) error {
	panic("implement me")
}

func (s Syscalls) VerifyConsensusFault(epoch abi.ChainEpoch, h1, h2 []byte) error {
	panic("implement me")
}
