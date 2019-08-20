package porcelain

import (
	"bytes"
	"context"
	"io"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/multiformats/go-multihash"

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

	sectorId, err := plumbing.SectorBuilder().AddPiece(ctx, node.Cid(), size, dagReader)
	if err != nil {
		return 0, errors.Wrap(err, "could not add piece")
	}

	return sectorId, nil
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
