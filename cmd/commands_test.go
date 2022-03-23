package cmd_test

import (
	"testing"

	"github.com/filecoin-project/venus/app/node/test"
)

func buildWithMiner(t *testing.T, builder *test.NodeBuilder) {
	// bundle together common init options for node test state
	cs := test.FixtureChainSeed(t)
	builder.WithGenesisInit(cs.GenesisInitFunc)
}
