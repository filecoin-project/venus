package consensus

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// ALPHA1 is the first alpha network
const ALPHA1 = "alpha1"

// DEVNET is the network name of devnet
const DEVNET4 = "devnet"

// LOCALNET is the network name of localnet
const LOCALNET = "localnet"

// TEST is the network name for internal tests
const TEST = "go-filecoin-test"

// Protocol0 is the first protocol version
const Protocol0 = 0

// ConfigureProtocolVersions configures all protocol upgrades for all known networks.
func ConfigureProtocolVersions(network string) *ProtocolUpgradeTable {
	return NewProtocolUpgradeTableBuilder(network).
		Add(ALPHA1, Protocol0, types.NewBlockHeight(0)).
		Add(DEVNET4, Protocol0, types.NewBlockHeight(0)).
		Add(LOCALNET, Protocol0, types.NewBlockHeight(0)).
		Add(TEST, Protocol0, types.NewBlockHeight(0)).
		Build()
}
