package ffiwrapper

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"
)

type Verifier interface {
	VerifySeal(proof7.SealVerifyInfo) (bool, error)
	VerifyAggregateSeals(aggregate proof7.AggregateSealVerifyProofAndInfos) (bool, error)
	VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) (bool, error)
	VerifyWinningPoSt(ctx context.Context, info proof7.WinningPoStVerifyInfo) (bool, error)
	VerifyWindowPoSt(ctx context.Context, info proof7.WindowPoStVerifyInfo) (bool, error)

	GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredPoStProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error)
}
