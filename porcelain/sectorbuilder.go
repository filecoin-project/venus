package porcelain

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/pkg/errors"
)

type sbPlumbing interface {
	SectorBuilder() sectorbuilder.SectorBuilder
	DAGImportData(context.Context, io.Reader) (ipld.Node, error)
	DAGCat(context.Context, cid.Cid) (io.Reader, error)
	DAGGetFileSize(ctx context.Context, c cid.Cid) (uint64, error)
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

// AddPiece adds piece data to a staged sector
func AddPiece(ctx context.Context, plumbing sbPlumbing, pieceReader io.Reader) (uint64, error) {
	if plumbing.SectorBuilder() == nil {
		return 0, errors.New("must be mining to add piece")
	}

	node, err := plumbing.DAGImportData(ctx, pieceReader)
	if err != nil {
		return 0, errors.Wrap(err, "could not read piece into local store")
	}

	dagReader, err := plumbing.DAGCat(ctx, node.Cid())
	if err != nil {
		return 0, errors.Wrap(err, "could not obtain reader for piece")
	}

	size, err := plumbing.DAGGetFileSize(ctx, node.Cid())
	if err != nil {
		return 0, errors.Wrap(err, "could not calculate piece size")
	}

	sectorID, err := plumbing.SectorBuilder().AddPiece(ctx, node.Cid(), size, dagReader)
	if err != nil {
		return 0, errors.Wrap(err, "could not add piece")
	}

	return sectorID, nil
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

	// if no sectors are staged, nothing to do
	if len(stagedSectors) == 0 {
		return nil
	}

	// start sealing on all existing staged sectors
	return plumbing.SectorBuilder().SealAllStagedSectors(ctx)
}
