package consensus

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	proof7 "github.com/filecoin-project/specs-actors/v7/actors/runtime/proof"

	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
)

type genFakeVerifier struct{}

var _ ffiwrapper.Verifier = (*genFakeVerifier)(nil)

func (m genFakeVerifier) VerifySeal(proof7.SealVerifyInfo) (bool, error) {
	return true, nil
}

func (m genFakeVerifier) VerifyAggregateSeals(proof7.AggregateSealVerifyProofAndInfos) (bool, error) {
	panic("implement me")
}

func (m genFakeVerifier) VerifyReplicaUpdate(update proof7.ReplicaUpdateInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) VerifyWinningPoSt(ctx context.Context, info proof7.WinningPoStVerifyInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) VerifyWindowPoSt(ctx context.Context, info proof7.WindowPoStVerifyInfo) (bool, error) {
	panic("not supported")
}

func (m genFakeVerifier) GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredPoStProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error) {
	panic("not supported")
}
