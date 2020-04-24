package consensus

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
)

// TestElectionPoster generates and verifies electoin PoSts
type TestElectionPoster struct{}

var _ EPoStVerifier = new(TestElectionPoster)
var _ postgenerator.PoStGenerator = new(TestElectionPoster)

// VerifyWinningPoSt returns the validity of the input PoSt proof
func (ep *TestElectionPoster) VerifyWinningPoSt(_ context.Context, _ abi.WinningPoStVerifyInfo) (bool, error) {
	return true, nil
}

// GenerateWinningPoStSectorChallenge determines the challenges used to create a winning PoSt.
func (ep *TestElectionPoster) GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error) {
	return nil, nil
}

// GenerateWinningPoSt creates a post proof for a winning block
func (ep *TestElectionPoster) GenerateWinningPoSt(ctx context.Context, minerID abi.ActorID, sectorInfo []abi.SectorInfo, randomness abi.PoStRandomness) ([]abi.PoStProof, error) {
	return []abi.PoStProof{{
		RegisteredProof: constants.DevRegisteredWinningPoStProof,
		ProofBytes:      []byte{0xe},
	}}, nil
}
