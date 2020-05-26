package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

type FakeSyscalls struct {
}

func (f FakeSyscalls) VerifySignature(ctx context.Context, view SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	// The signer is assumed to be already resolved to a pubkey address.
	return crypto.ValidateSignature(plaintext, signer, signature)
}

func (f FakeSyscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

func (f FakeSyscalls) ComputeUnsealedSectorCID(ctx context.Context, proof abi.RegisteredProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	panic("implement me")
}

func (f FakeSyscalls) VerifySeal(ctx context.Context, info abi.SealVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyWinningPoSt(ctx context.Context, info abi.WinningPoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyPoSt(ctx context.Context, info abi.WindowPoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, head block.TipSetKey, view SyscallsStateView) (*runtime.ConsensusFault, error) {
	panic("implement me")
}
