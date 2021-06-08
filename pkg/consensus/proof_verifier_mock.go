package consensus

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	proof5 "github.com/filecoin-project/specs-actors/v5/actors/runtime/proof"

	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
)

type genFakeVerifier struct{}

var _ ffiwrapper.Verifier = (*genFakeVerifier)(nil)

func (m genFakeVerifier) VerifySeal(svi proof5.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (m genFakeVerifier) VerifyAggregateSeals(aggregate proof5.AggregateSealVerifyProofAndInfos) (bool, error) {
	panic("implement me")
}

func (m genFakeVerifier) VerifyWinningPoSt(ctx context.Context, info proof5.WinningPoStVerifyInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) VerifyWindowPoSt(ctx context.Context, info proof5.WindowPoStVerifyInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) GenerateWinningPoStSectorChallenge(ctx context.Context, proof abi.RegisteredPoStProof, id abi.ActorID, randomness abi.PoStRandomness, u uint64) ([]uint64, error) {
	panic("not supported")
}
