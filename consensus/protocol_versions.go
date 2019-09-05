package consensus

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// ALPHA1 is the first alpha network
const ALPHA1 = "alpha1"

// DEVNET is the network name of devnet
const DEVNET = "devnet"

// LOCALNET is the network name of localnet
const LOCALNET = "localnet"

// TEST is the network name for internal tests
const TEST = "go-filecoin-test"

// Protocol0 is the first protocol version
const Protocol0 = 0

// ConfigureProtocolVersions configures all protocol upgrades for all known networks.
func ConfigureProtocolVersions(put *ProtocolUpgradeTable) {
	put.Add(ALPHA1, Protocol0, types.NewBlockHeight(0))

	put.Add(DEVNET, Protocol0, types.NewBlockHeight(0))

	put.Add(LOCALNET, Protocol0, types.NewBlockHeight(0))

	put.Add(TEST, Protocol0, types.NewBlockHeight(0))
}
