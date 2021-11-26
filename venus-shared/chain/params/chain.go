package params

import (
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
)

var BlocksPerEpoch = uint64(builtin0.ExpectedLeadersPerEpoch)
var MaxWinCount = 3 * int64(BlocksPerEpoch)

// ///////
// Limits

// TODO: If this is gonna stay, it should move to specs-actors
const BlockMessageLimit = 10000

const BlockGasLimit = 10_000_000_000
const BlockGasTarget = BlockGasLimit / 2
const BaseFeeMaxChangeDenom = 8 // 12.5%
const InitialBaseFee = 100e6
const MinimumBaseFee = 100
const PackingEfficiencyNum = 4
const PackingEfficiencyDenom = 5
