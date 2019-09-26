package version

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// USER is the user network
const USER = "alpha2"

// DEVNET4 is the network name of devnet
const DEVNET4 = "devnet4"

// LOCALNET is the network name of localnet
const LOCALNET = "localnet"

// TEST is the network name for internal tests
const TEST = "go-filecoin-test"

// Protocol0 is the first protocol version
const Protocol0 = 0

// ConfigureProtocolVersions configures all protocol upgrades for all known networks.
// TODO: support arbitrary network names at "latest" protocol version so that only coordinated
// network upgrades need to be represented here. See #3491.
func ConfigureProtocolVersions(network string) (*ProtocolVersionTable, error) {
	return NewProtocolVersionTableBuilder(network).
		Add(USER, Protocol0, types.NewBlockHeight(0)).
		Add(DEVNET4, Protocol0, types.NewBlockHeight(0)).
		Add(LOCALNET, Protocol0, types.NewBlockHeight(0)).
		Add(TEST, Protocol0, types.NewBlockHeight(0)).
		Build()
}
