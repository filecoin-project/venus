package constants

import (
	"math/big"

	"github.com/filecoin-project/venus/venus-shared/actors/policy"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-state-types/network"
)

/* inline-gen template

const TestNetworkVersion = network.Version{{.latestNetworkVersion}}

/* inline-gen start */

const TestNetworkVersion = network.Version25

/* inline-gen end */

// constants for Weight calculation
// The ratio of weight contributed by short-term vs long-term factors in a given round
const (
	WRatioNum = int64(1)
	WRatioDen = uint64(2)
)

const (
	FilBase = uint64(2_000_000_000)
)

const (
	FilecoinPrecision = uint64(1_000_000_000_000_000_000)
)

var InitialRewardBalance = WholeFIL(1_100_000_000)
var InitialFilReserved = WholeFIL(300_000_000)

func SetAddressNetwork(n address.Network) {
	address.CurrentNetwork = n
}

// assuming 4000 messages per round, this lets us not lose any messages across a
// 10 block reorg.
const BlsSignatureCacheSize = 40000

// Epochs
const ForkLengthThreshold = Finality

// Size of signature verification cache
// 32k keeps the cache around 10MB in size, max
const (
	VerifSigCacheSize = 32000
	Finality          = policy.ChainFinality
)

// Epochs
const MessageConfidence = uint64(5)

func WholeFIL(whole uint64) *big.Int {
	bigWhole := big.NewInt(int64(whole))
	return bigWhole.Mul(bigWhole, big.NewInt(int64(FilecoinPrecision)))
}
