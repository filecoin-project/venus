package postgenerator

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-storage/storage"
)

// PoStGenerator defines a method set used to generate PoSts
type PoStGenerator interface {
	GenerateEPostCandidates(ctx context.Context, minerID abi.ActorID, sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, faults []abi.SectorNumber) ([]storage.PoStCandidateWithTicket, error)
	ComputeElectionPoSt(ctx context.Context, minerID abi.ActorID, sectorInfo []abi.SectorInfo, challengeSeed abi.PoStRandomness, winners []abi.PoStCandidate) ([]abi.PoStProof, error)
}
