package params

import (
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
)

var BlocksPerEpoch = uint64(builtin0.ExpectedLeadersPerEpoch)
var MaxWinCount = 3 * int64(BlocksPerEpoch)
