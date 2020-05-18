package commands_test

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/fixtures/fortest"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node/test"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
)

// create a basic new TestDaemon, with a miner and the KeyInfo it needs to sign
// tickets and blocks. This does not set a DefaultAddress in the Wallet; in this
// case, node/init.go Init generates a new address in the wallet and sets it to
// the default address.
func makeTestDaemonWithMinerAndStart(t *testing.T) *th.TestDaemon {
	daemon := th.NewDaemon(
		t,
		th.WithMiner(fortest.TestMiners[0]),
		th.KeyFile(fortest.KeyFilePaths()[0]),
	).Start()
	return daemon
}

func buildWithMiner(t *testing.T, builder *test.NodeBuilder) {
	// bundle together common init options for node test state
	cs := node.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)
	builder.WithConfig(cs.MinerConfigOpt(0))
	builder.WithInitOpt(cs.KeyInitOpt(0))
}
