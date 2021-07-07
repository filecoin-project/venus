package ffiwrapper

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	proof5 "github.com/filecoin-project/specs-actors/v5/actors/runtime/proof"
)

type Verifier interface {
	VerifySeal(proof5.SealVerifyInfo) (bool, error)
	VerifyAggregateSeals(aggregate proof5.AggregateSealVerifyProofAndInfos) (bool, error)
	VerifyWinningPoSt(ctx context.Context, info proof5.WinningPoStVerifyInfo) (bool, error)
	VerifyWindowPoSt(ctx context.Context, info proof5.WindowPoStVerifyInfo) (bool, error)

	GenerateWinningPoStSectorChallenge(context.Context, abi.RegisteredPoStProof, abi.ActorID, abi.PoStRandomness, uint64) ([]uint64, error)
}
