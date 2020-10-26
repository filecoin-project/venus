package constants

import "github.com/filecoin-project/go-state-types/abi"

const DevSealProofType = abi.RegisteredSealProof_StackedDrg2KiBV1

// DevSectorSize is a tiny sector useful only for testing.
var DevSectorSize abi.SectorSize

func init() {
	var err error
	DevSectorSize, err = DevSealProofType.SectorSize()
	if err != nil {
		panic(err)
	}
}

// FiveHundredTwelveMiBSectorSize contain 512MiB after sealing.
var FiveHundredTwelveMiBSectorSize = abi.SectorSize(512 << 20)

// ThirtyTwoGiBSectorSize contain 32GiB after sealing.
var ThirtyTwoGiBSectorSize = abi.SectorSize(1 << 35)

// EightMiBSectorSize contains 8MiB after sealing.
var EightMiBSectorSize = abi.SectorSize(1 << 23)
