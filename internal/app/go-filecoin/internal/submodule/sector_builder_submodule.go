package submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
)

// SectorBuilderSubmodule enhances the `Node` with sector storage capabilities.
type SectorBuilderSubmodule struct {
	// SectorBuilder is used by the miner to fill and seal sectors.
	SectorBuilder  sectorbuilder.SectorBuilder
	ElectionPoster *proofs.ElectionPoster
}

// NewSectorStorageSubmodule creates a new sector builder submodule.
func NewSectorStorageSubmodule(ctx context.Context) (SectorBuilderSubmodule, error) {
	return SectorBuilderSubmodule{
		ElectionPoster: &proofs.ElectionPoster{},
		// sectorBuilder: nil,
	}, nil
}
