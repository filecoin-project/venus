package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	proof5 "github.com/filecoin-project/specs-actors/v5/actors/runtime/proof"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/venus/pkg/crypto"
)

type FakeSyscalls struct {
}

func (f FakeSyscalls) VerifySignature(ctx context.Context, view SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	// The signer is assumed To be already resolved To a pubkey address.
	return crypto.ValidateSignature(plaintext, signer, signature)
}

func (f FakeSyscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

func (f FakeSyscalls) ComputeUnsealedSectorCID(ctx context.Context, proof5 abi.RegisteredSealProof, pieces []abi.PieceInfo) (cid.Cid, error) {
	panic("implement me")
}

func (f FakeSyscalls) VerifySeal(ctx context.Context, info proof5.SealVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) BatchVerifySeals(ctx context.Context, vis map[address.Address][]proof5.SealVerifyInfo) (map[address.Address][]bool, error) {
	panic("implement me")
}

func (f FakeSyscalls) VerifyWinningPoSt(ctx context.Context, info proof5.WinningPoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyPoSt(ctx context.Context, info proof5.WindowPoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, view SyscallsStateView) (*rt5.ConsensusFault, error) {
	panic("implement me")
}
