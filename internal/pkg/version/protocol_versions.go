package version

import (
	"github.com/filecoin-project/go-state-types/abi"
)

// TEST is the network name for internal tests
const TEST = "gfctest"

// Protocol0 is the first protocol version
const Protocol0 = 0

// ConfigureProtocolVersions configures all protocol upgrades for all known networks.
// TODO: support arbitrary network names at "latest" protocol version so that only coordinated
// network upgrades need to be represented here. See #3491.
func ConfigureProtocolVersions(network string) (*ProtocolVersionTable, error) {
	return NewProtocolVersionTableBuilder(network).
		Add("testnetnet", Protocol0, abi.ChainEpoch(0)).
		Add("mainnet", Protocol0, abi.ChainEpoch(0)).
		Add("localnet", Protocol0, abi.ChainEpoch(0)).
		Add(TEST, Protocol0, abi.ChainEpoch(0)).
		Build()
}
