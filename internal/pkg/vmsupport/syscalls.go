package vmsupport

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/sector-storage/ffiwrapper"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/slashing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
)

type faultChecker interface {
	VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, head block.TipSetKey, view slashing.FaultStateView, earliest abi.ChainEpoch) (*runtime.ConsensusFault, error)
}

// Syscalls contains the concrete implementation of VM system calls, including connection to
// proof verification and blockchain inspection.
// Errors returned by these methods are intended to be returned to the actor code to respond to: they must be
// entirely deterministic and repeatable by other implementations.
// Any non-deterministic error will instead trigger a panic.
// TODO: determine a more robust mechanism for distinguishing transient runtime failures from deterministic errors
// in VM and supporting code. https://github.com/filecoin-project/go-filecoin/issues/3844
type Syscalls struct {
	faultChecker faultChecker
	verifier     ffiwrapper.Verifier
}

func NewSyscalls(faultChecker faultChecker, verifier ffiwrapper.Verifier) *Syscalls {
	return &Syscalls{
		faultChecker: faultChecker,
		verifier:     verifier,
	}
}

func (s *Syscalls) VerifySignature(ctx context.Context, view vm.SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	return state.NewSignatureValidator(view).ValidateSignature(ctx, plaintext, signer, signature)
}

func (s *Syscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

func (s *Syscalls) ComputeUnsealedSectorCID(_ context.Context, proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	return ffiwrapper.GenerateUnsealedCID(proof, pieces)
}

func (s *Syscalls) VerifySeal(_ context.Context, info abi.SealVerifyInfo) error {
	ok, err := s.verifier.VerifySeal(info)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("seal invalid")
	}
	return nil
}

func (s *Syscalls) VerifyPoSt(ctx context.Context, info abi.PoStVerifyInfo) error {
	ok, err := s.verifier.VerifyFallbackPost(ctx, info)
	if err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("proof invalid")
	}
	return nil
}

func (s *Syscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, head block.TipSetKey, view vm.SyscallsStateView, earliest abi.ChainEpoch) (*runtime.ConsensusFault, error) {
	return s.faultChecker.VerifyConsensusFault(ctx, h1, h2, extra, head, view, earliest)
}
