package postgenerator

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"
)

// PoStGenerator defines a method set used to generate PoSts
type PoStGenerator interface {
	GenerateWinningPoSt(ctx context.Context, minerID abi.ActorID, sectorInfo []abi.SectorInfo, randomness abi.PoStRandomness) ([]abi.PoStProof, error)
}
