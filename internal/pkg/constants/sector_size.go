package constants

import "github.com/filecoin-project/specs-actors/actors/abi"

// DevSectorSize is a tiny sector useful only for testing.
var DevSectorSize = abi.SectorSize(2048)

// TwoHundredFiftySixMiBSectorSize contain 256MiB after sealing.
var FiveHundredTwelveMiBSectorSize = abi.SectorSize(512 << 20)
