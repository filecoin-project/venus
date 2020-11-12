package consensus

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
)

// TestElectionPoster generates and verifies electoin PoSts
type TestElectionPoster struct{}

//var _ EPoStVerifier = new(TestElectionPoster)
//var _ postgenerator.PoStGenerator = new(TestElectionPoster)
//

func (ep *TestElectionPoster) GenerateWinningPoSt(ctx context.Context,
	minerID abi.ActorID,
	sectorInfo []builtin.SectorInfo,
	randomness abi.PoStRandomness) ([]builtin.PoStProof, error) {
	return []builtin.PoStProof{{
		PoStProof:  constants.DevRegisteredWinningPoStProof,
		ProofBytes: []byte{0xe},
	}}, nil
}

//// VerifyWinningPoSt returns the validity of the input PoSt proof
//func (ep *TestElectionPoster) VerifyWinningPoSt(_ context.Context, _ abi.WinningPoStVerifyInfo) (bool, error) {
//	return true, nil
//}
//
//// GenerateWinningPoStSectorChallenge determines the challenges used to create a winning PoSt.
//func (ep *TestElectionPoster) GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredSealProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error) {
//	return nil, nil
//}
//
