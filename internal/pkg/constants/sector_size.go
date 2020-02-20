package constants

import "github.com/filecoin-project/specs-actors/actors/abi"

// DevSectorSize is a tiny sector useful only for testing.
var DevSectorSize = abi.SectorSize(1024)

// TwoHundredFiftySixMiBSectorSize contain 256MiB after sealing.
var TwoHundredFiftySixMiBSectorSize = abi.SectorSize(1 << 28)
