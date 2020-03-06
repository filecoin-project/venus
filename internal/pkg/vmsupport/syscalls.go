package vmsupport

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

type Syscalls struct {
	verifier sectorbuilder.Verifier
}

func NewSyscalls(verifier sectorbuilder.Verifier) *Syscalls {
	return &Syscalls{
		verifier: verifier,
	}
}

func (s Syscalls) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) error {
	if signer.Protocol() != address.SECP256K1 && signer.Protocol() != address.BLS {
		// This is a programmer error: callers must resolve the address in state first.
		return fmt.Errorf("unresolved signer address")
	}
	return crypto.ValidateSignature(plaintext, signer, signature)
}

func (s Syscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

func (s Syscalls) ComputeUnsealedSectorCID(_ context.Context, proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	return sectorbuilder.GenerateUnsealedCID(proof, pieces)
}

func (s Syscalls) VerifySeal(ctx context.Context, info abi.SealVerifyInfo) error {
	ok, err := s.verifier.VerifySeal(info)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("seal invalid")
	}
	return nil
}

func (s Syscalls) VerifyPoSt(ctx context.Context, info abi.PoStVerifyInfo) error {
	ok, err := s.verifier.VerifyFallbackPost(ctx, info)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("proof invalid")
	}
	return nil
}

func (s Syscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte) (*runtime.ConsensusFault, error) {
	panic("implement me")
}
