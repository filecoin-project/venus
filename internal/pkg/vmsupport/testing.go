package vmsupport

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"

	gfcrypto "github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
)

type FakeSyscalls struct {
}

func (f FakeSyscalls) VerifySignature(epoch abi.ChainEpoch, signature gfcrypto.Signature, signer address.Address, plaintext []byte) error {
	// This doesn't resolve account ID addresses to their signing addresses (but should).
	return gfcrypto.ValidateSignature(plaintext, signer, signature)
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

func (f FakeSyscalls) VerifyPoSt(ctx context.Context, info abi.PoStVerifyInfo) error {
	panic("implement me")
}

func (f FakeSyscalls) VerifyConsensusFault(ctx context.Context, h1, h2, extra []byte) (*runtime.ConsensusFault, error) {
	panic("implement me")
}
