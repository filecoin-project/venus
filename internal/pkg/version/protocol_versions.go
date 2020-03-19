package version

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// USER is the user network
const USER = "alpha2"

// INTEROP is the network name of an interop net
const INTEROP = "interop"

// LOCALNET is the network name of localnet
const LOCALNET = "localnet"

// TEST is the network name for internal tests
const TEST = "gfctest"

// Protocol0 is the first protocol version
const Protocol0 = 0

// ConfigureProtocolVersions configures all protocol upgrades for all known networks.
// TODO: support arbitrary network names at "latest" protocol version so that only coordinated
// network upgrades need to be represented here. See #3491.
func ConfigureProtocolVersions(network string) (*ProtocolVersionTable, error) {
	return NewProtocolVersionTableBuilder(network).
		Add(USER, Protocol0, abi.ChainEpoch(0)).
		Add(INTEROP, Protocol0, abi.ChainEpoch(0)).
		Add(LOCALNET, Protocol0, abi.ChainEpoch(0)).
		Add(TEST, Protocol0, abi.ChainEpoch(0)).
		Build()
}
