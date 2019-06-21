package porcelain

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/types"
)

type sbPlumbing interface {
	SectorBuilder() sectorbuilder.SectorBuilder
}

// CalculatePoSt invokes the sector builder to calculate a proof-of-spacetime.
func CalculatePoSt(ctx context.Context, plumbing sbPlumbing, sortedCommRs proofs.SortedCommRs, seed types.PoStChallengeSeed) ([]types.PoStProof, []uint64, error) {
	req := sectorbuilder.GeneratePoStRequest{
		SortedCommRs:  sortedCommRs,
		ChallengeSeed: seed,
	}
	sb := plumbing.SectorBuilder()
	if sb == nil {
		return nil, nil, errors.New("no sector builder initialised")
	}
	res, err := sb.GeneratePoSt(req)
	if err != nil {
		return nil, nil, err
	}
	return res.Proofs, res.Faults, nil
}
