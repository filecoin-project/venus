package constants

import "github.com/filecoin-project/go-state-types/abi"

// just for test
var DevRegisteredSealProof = abi.RegisteredSealProof_StackedDrg2KiBV1

var (
	DevRegisteredWinningPoStProof = abi.RegisteredPoStProof_StackedDrgWinning2KiBV1
	DevRegisteredWindowPoStProof  = abi.RegisteredPoStProof_StackedDrgWindow2KiBV1
)
