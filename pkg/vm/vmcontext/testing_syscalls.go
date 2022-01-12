package vmcontext

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	rt7 "github.com/filecoin-project/specs-actors/v7/actors/runtime"
	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"

	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	"github.com/filecoin-project/venus/pkg/crypto"
)

type FakeSyscalls struct {
}

func (f FakeSyscalls) VerifySignature(ctx context.Context, view SyscallsStateView, signature crypto.Signature, signer address.Address, plaintext []byte) error {
	// The signer is assumed To be already resolved To a pubkey address.
	return crypto.Verify(&signature, signer, plaintext)
}

func (f FakeSyscalls) HashBlake2b(data []byte) [32]byte {
	return blake2b.Sum256(data)
}

func (f FakeSyscalls) ComputeUnsealedSectorCID(context.Context, abi.RegisteredSealProof, []abi.PieceInfo) (cid.Cid, error) {
	panic("implement me")
}

func (f FakeSyscalls) VerifySeal(ctx context.Context, info proof7.SealVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) BatchVerifySeals(ctx context.Context, vis map[address.Address][]proof7.SealVerifyInfo) (map[address.Address][]bool, error) {
	panic("implement me")
}

func (f FakeSyscalls) VerifyWinningPoSt(ctx context.Context, info proof7.WinningPoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyWindowPoSt(ctx context.Context, info proof7.WindowPoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte, view SyscallsStateView) (*rt7.ConsensusFault, error) {
	panic("implement me")
}
