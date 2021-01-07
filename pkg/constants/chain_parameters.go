package constants

import (
	"github.com/filecoin-project/go-state-types/abi"
	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
)

const DefaultConfidence = abi.ChainEpoch(5)
const DefaultMessageWaitLookback = abi.ChainEpoch(100) // in most cases, this should be enough to avoid races.
const LookbackNoLimit = abi.ChainEpoch(-1)

const BlockMessageLimit = 10000

// Epochs
const TicketRandomnessLookback = abi.ChainEpoch(1)

//expect blocks number in a tipset
var ExpectedLeadersPerEpoch = builtin0.ExpectedLeadersPerEpoch

// BlockGasLimit is the maximum amount of gas that can be used to execute messages in a single block.
const BlockGasLimit = 10_000_000_000
const BlockGasTarget = BlockGasLimit / 2
const BaseFeeMaxChangeDenom = 8 // 12.5%
const InitialBaseFee = 100e6
const MinimumBaseFee = 100
const PackingEfficiencyNum = 4
const PackingEfficiencyDenom = 5

const BlockDelaySecs = uint64(builtin0.EpochDurationSeconds)

const PropagationDelaySecs = uint64(6)

var InsecurePoStValidation = false

// the 'f' prefix doesn't matter
var ZeroAddress = MustParseAddress("f3yaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaby2smx7a")

var DefaultDefaultMaxFee = MustParseFIL("0.007")
var DefaultSimultaneousTransfers = uint64(20)
