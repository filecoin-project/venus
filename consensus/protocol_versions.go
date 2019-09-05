package consensus

import (
	"github.com/filecoin-project/go-filecoin/types"
)

const ALPHA1 = "alpha1"
const DEVNET = "devnet"
const LOCALNET = "localnet"
const TEST = "go-filecoin-test"

const protocol_0 = 0

func ConfigureProtocolVersions(put *ProtocolUpgradeTable) {
	put.Add(ALPHA1, protocol_0, types.NewBlockHeight(0))

	put.Add(DEVNET, protocol_0, types.NewBlockHeight(0))

	put.Add(LOCALNET, protocol_0, types.NewBlockHeight(0))

	put.Add(TEST, protocol_0, types.NewBlockHeight(0))
}
