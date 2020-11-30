package chain

import (
	"github.com/filecoin-project/go-state-types/abi"
	"time"
)

// SectorInfo provides information about a sector construction
type SectorInfo struct {
	Size         abi.SectorSize
	MaxPieceSize abi.UnpaddedPieceSize
}

// ProtocolParams contains parameters that modify the filecoin nodes protocol
type ProtocolParams struct {
	Network          string
	BlockTime        time.Duration
	SupportedSectors []SectorInfo
}
