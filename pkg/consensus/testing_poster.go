package consensus

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
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
