package params

import (
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
)

var (
	BlocksPerEpoch = uint64(builtin0.ExpectedLeadersPerEpoch)
	MaxWinCount    = 3 * int64(BlocksPerEpoch)
)

// ///////
// Limits

// TODO: If this is gonna stay, it should move to specs-actors
const BlockMessageLimit = 10000

const (
	BlockGasLimit          = 10_000_000_000
	BlockGasTarget         = BlockGasLimit / 2
	BaseFeeMaxChangeDenom  = 8 // 12.5%
	InitialBaseFee         = 100e6
	MinimumBaseFee         = 100
	PackingEfficiencyNum   = 4
	PackingEfficiencyDenom = 5
)
