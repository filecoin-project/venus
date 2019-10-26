package sectorbuilder

import (
	"context"
	"io"

	go_sectorbuilder "github.com/filecoin-project/go-sectorbuilder"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// SectorBuilder provides an interface through which user piece-bytes can be
// written, sealed into sectors, and later unsealed and read.
type SectorBuilder interface {
	// AddPiece writes the given piece into an unsealed sector and returns the
	// id of that sector. This method has a race; it is possible that the
	// sector into which the piece-bytes were written is sealed before this
	// method returns. In the real world this should not happen, as sealing
	// takes a long time to complete. In tests, where sealing happens
	// near-instantaneously, it is possible to exercise this race.
	AddPiece(ctx context.Context, pieceRef cid.Cid, pieceSize uint64, pieceReader io.Reader) (sectorID uint64, err error)

	// ReadPieceFromSealedSector produces a Reader used to get original
	// piece-bytes from a sealed sector.
	ReadPieceFromSealedSector(pieceCid cid.Cid) (io.Reader, error)

	// SealAllStagedSectors seals any non-empty staged sectors.
	SealAllStagedSectors(ctx context.Context) error

	// GetAllStagedSectors returns a slice of all staged sector metadata for the sector builder, or an error.
	GetAllStagedSectors() ([]go_sectorbuilder.StagedSectorMetadata, error)

	// SectorSealResults returns an unbuffered channel that is sent a value
	// whenever sealing completes. All calls to SectorSealResults will get the
	// same channel. Values will be either a *SealedSectorMetadata or an error.
	// A *SealedSectorMetadata will be sent to the returned channel only once,
	// regardless of the number of times SectorSealResults is called.
	SectorSealResults() <-chan SectorSealResult

	// GeneratePoSt creates a proof-of-spacetime for the replicas managed by
	// the SectorBuilder. Its output includes the proof-of-spacetime proof which
	// is posted to the blockchain along with any faults. The proof can be
	// verified by the VerifyPoSt method on the Verifier interface.
	GeneratePoSt(GeneratePoStRequest) (GeneratePoStResponse, error)

	// Close signals that this SectorBuilder is no longer in use. SectorBuilder
	// metadata will not be deleted when Close is called; an equivalent
	// SectorBuilder can be created later by applying the Init function to the
	// arguments used to create the instance being closed.
	Close() error
}

// SectorSealResult represents the outcome of a sector's sealing.
type SectorSealResult struct {
	SectorID uint64

	// SealingErr contains any error encountered while sealing.
	// Note: Either SealingResult or SealingErr may be non-nil, not both.
	SealingErr error

	// SealingResult contains the successful output of the sealing operation.
	// Note: Either SealingResult or SealingErr may be non-nil, not both.
	SealingResult *SealedSectorMetadata
}

// PieceInfo is information about a filecoin piece
type PieceInfo struct {
	Ref            cid.Cid     `json:"ref"`
	Size           uint64      `json:"size"` // TODO: use BytesAmount
	InclusionProof []byte      `json:"inclusionProof"`
	CommP          types.CommP `json:"commP"`
}

// SealedSectorMetadata is a sector that has been sealed by the PoRep setup process
type SealedSectorMetadata struct {
	CommD     types.CommD
	CommR     types.CommR // deprecated (will be removed soon)
	CommRStar types.CommRStar
	Pieces    []*PieceInfo // deprecated (will be removed soon)
	Proof     types.PoRepProof
	SectorID  uint64
}

// GeneratePoStRequest represents a request to generate a proof-of-spacetime.
type GeneratePoStRequest struct {
	SortedSectorInfo go_sectorbuilder.SortedSectorInfo
	ChallengeSeed    types.PoStChallengeSeed
}

// GeneratePoStResponse contains PoST proof.
type GeneratePoStResponse struct {
	Proof types.PoStProof
}
