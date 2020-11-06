package constants

import (
	"github.com/filecoin-project/go-state-types/abi"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
)

const FilBase = uint64(2_000_000_000)
const BlockMessageLimit = 10000

// Epochs
const TicketRandomnessLookback = abi.ChainEpoch(1)

//expect blocks number in a tipset
var ExpectedLeadersPerEpoch = builtin0.ExpectedLeadersPerEpoch
