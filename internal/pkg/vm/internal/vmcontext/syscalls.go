package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
)

// Syscall implementation interface.
// These methods take the chain epoch and other context that is implicit in the runtime as explicit parameters.
type SyscallsImpl interface {
	VerifySignature(epoch abi.ChainEpoch, signature crypto.Signature, signer address.Address, plaintext []byte) error
	HashBlake2b(data []byte) [32]byte
	ComputeUnsealedSectorCID(ctx context.Context, proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error)
	VerifySeal(ctx context.Context, info abi.SealVerifyInfo) error
	VerifyPoSt(ctx context.Context, info abi.PoStVerifyInfo) error
	VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte) (*specsruntime.ConsensusFault, error)
}

type syscalls struct {
	impl      SyscallsImpl
	ctx       context.Context
	gasTank   *GasTracker
	pricelist gascost.Pricelist
	epoch     abi.ChainEpoch
}

var _ specsruntime.Syscalls = (*syscalls)(nil)

func (sys syscalls) VerifySignature(signature crypto.Signature, signer address.Address, plaintext []byte) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifySignature(signature.Type, len(plaintext)))
	return sys.impl.VerifySignature(sys.epoch, signature, signer, plaintext)
}

func (sys syscalls) HashBlake2b(data []byte) [32]byte {
	sys.gasTank.Charge(sys.pricelist.OnHashing(len(data)))
	return sys.impl.HashBlake2b(data)
}

func (sys syscalls) ComputeUnsealedSectorCID(proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	sys.gasTank.Charge(sys.pricelist.OnComputeUnsealedSectorCid(proof, &pieces))
	return sys.impl.ComputeUnsealedSectorCID(sys.ctx, proof, pieces)
}

func (sys syscalls) VerifySeal(info abi.SealVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifySeal(info))
	return sys.impl.VerifySeal(sys.ctx, info)
}

func (sys syscalls) VerifyPoSt(info abi.PoStVerifyInfo) error {
	sys.gasTank.Charge(sys.pricelist.OnVerifyPost(info))
	return sys.impl.VerifyPoSt(sys.ctx, info)
}

func (sys syscalls) VerifyConsensusFault(h1, h2, extra []byte) (*specsruntime.ConsensusFault, error) {
	sys.gasTank.Charge(sys.pricelist.OnVerifyConsensusFault())
	return sys.impl.VerifyConsensusFault(sys.ctx, h1, h2, extra)
}
