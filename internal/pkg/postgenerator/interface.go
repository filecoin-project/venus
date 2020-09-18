package postgenerator

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/proof"
)

// PoStGenerator defines a method set used to generate PoSts
type PoStGenerator interface {
	GenerateWinningPoSt(ctx context.Context, minerID abi.ActorID, sectorInfo []proof.SectorInfo, randomness abi.PoStRandomness) ([]proof.PoStProof, error)
}
