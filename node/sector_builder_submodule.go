package node

import "github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"

// SectorBuilderSubmodule enhances the `Node` with sector storage capabilities.
type SectorBuilderSubmodule struct {
	// SectorBuilder is used by the miner to fill and seal sectors.
	sectorBuilder sectorbuilder.SectorBuilder
}
