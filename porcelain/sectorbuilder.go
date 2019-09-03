package porcelain

import (
	"bytes"
	"context"

	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"

	"github.com/pkg/errors"
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

// SealNow forces the sectorbuilder to either seal the staged sectors it has or create a new one and seal it immediately
func SealNow(ctx context.Context, plumbing sbPlumbing) error {
	if plumbing.SectorBuilder() == nil {
		return errors.New("must be mining to seal sectors")
	}

	stagedSectors, err := plumbing.SectorBuilder().GetAllStagedSectors()
	if err != nil {
		return errors.Wrap(err, "could not retrieved staged sectors")
	}

	// if no sectors are staged, add a 1 byte piece to ensure at least one seal
	if len(stagedSectors) == 0 {
		data := []byte{0}
		hash, err := multihash.Sum(data, multihash.SHA2_256, -1)
		if err != nil {
			return errors.Wrap(err, "could not create cid for piece")
		}
		pieceRef := cid.NewCidV1(cid.DagCBOR, hash)
		_, err = plumbing.SectorBuilder().AddPiece(ctx, pieceRef, 1, bytes.NewReader(data))
		if err != nil {
			return errors.Wrap(err, "could not add piece to trigger sealing")
		}

	}

	// start sealing on all existing staged sectors
	return plumbing.SectorBuilder().SealAllStagedSectors(ctx)
}
