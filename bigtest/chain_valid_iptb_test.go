package bigtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestMiningChain(t *testing.T) {
	// must pass "-bigtest=true" flag to run
	if !*bigtest {
		t.SkipNow()
	}
	require := require.New(t)
	assert := assert.New(t)

	// things to fiddle with
	numNodes := 50
	chainPropDuration := 5 * time.Second

	// Start and Connect numNode
	nodes := MustStartIPTB(require, numNodes, "filecoin", "local")
	defer MustKillIPTB(require, nodes)

	MustConnectIPTB(require, nodes)

	for cl, n := range nodes {
		MustRunIPTB(require, n, "go-filecoin", "mining", "once")
		MustHaveChainLengthBy(require, cl+2, chainPropDuration, nodes)
	}

	// get every nodes chain, validate their lengths
	nodeChains := make(map[int][]types.Block)
	for i, n := range nodes {
		// get the chain from the node as json
		out := MustRunIPTB(require, n, "go-filecoin", "chain", "ls", "--enc=json")

		// unmarshal the json string into a "list of blocks"
		chain := MustUnmarshalChain(require, out)

		nodeChains[i] = chain
	}
	expected := nodeChains[0][0]
	// ensure all the chains are equivalent
	MustHaveEqualChainHeads(assert, expected, nodeChains)
}
