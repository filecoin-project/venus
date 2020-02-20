package types

import "github.com/filecoin-project/specs-actors/actors/abi"

// OneKiBSectorSize contain 1024 bytes after sealing. These sectors are used
// when the network is in TestProofsMode.
var OneKiBSectorSize = abi.SectorSize(1024)

// TwoHundredFiftySixMiBSectorSize contain 256MiB after sealing. These sectors
// are used when the network is in LiveProofsMode.
var TwoHundredFiftySixMiBSectorSize = abi.SectorSize(1 << 28)
