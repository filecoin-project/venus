package consensus

import (
	"context"

	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof" // todo ref lotus

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/internal/pkg/util/ffiwrapper"
)

type genFakeVerifier struct{}

var _ ffiwrapper.Verifier = (*genFakeVerifier)(nil)

func (m genFakeVerifier) VerifySeal(svi proof2.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (m genFakeVerifier) VerifyWinningPoSt(ctx context.Context, info proof2.WinningPoStVerifyInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) VerifyWindowPoSt(ctx context.Context, info proof2.WindowPoStVerifyInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) GenerateWinningPoStSectorChallenge(ctx context.Context, proof abi.RegisteredPoStProof, id abi.ActorID, randomness abi.PoStRandomness, u uint64) ([]uint64, error) {
	panic("not supported")
}
