package commands_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/fixtures"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
)

// create a basic new TestDaemon, with a miner and the KeyInfo it needs to sign
// tickets and blocks. This does not set a DefaultAddress in the Wallet; in this
// case, node/init.go Init generates a new address in the wallet and sets it to
// the default address.
func makeTestDaemonWithMinerAndStart(t *testing.T) *th.TestDaemon {
	daemon := th.NewDaemon(
		t,
		th.WithMiner(fixtures.TestMiners[0]),
		th.KeyFile(fixtures.KeyFilePaths()[0]),
	).Start()
	return daemon
}
