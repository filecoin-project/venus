package bigtest

import (
	"bytes"
	"encoding/json"
	"flag"
	"strings"
	"sync"
	"testing"
	"time"

	iptb "gx/ipfs/QmPW6YejVF5nQctt9QSSBaTPpvPgC85QsPxCNzVAjL6eo9/iptb/util"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/types"
)

var bigtest = flag.Bool("bigtest", false, "used to run the big tests")

func TestMiningChain(t *testing.T) {
	// to run the bigtest's:
	// `go test run -bigtest=true ./...`
	if !*bigtest {
		t.SkipNow()
	}
	// if anything errors we should fail immediately
	require := require.New(t)

	// Tested 100 locally with 16gm RAM, took 8 mins
	numNodes := 15

	// initialize numNodes filecoin nodes locally and start them
	cfg := &iptb.InitCfg{
		Count:      numNodes,
		NodeType:   "filecoin",
		DeployType: "local",
		Force:      true,
		Bootstrap:  "skip",
	}
	err := iptb.TestbedInit(cfg)
	require.NoError(err)

	nodes, err := iptb.LoadNodes()
	require.NoError(err)

	err = iptb.TestbedStart(nodes, false, []string{})
	require.NoError(err)

	defer func() {
		// Be sure to kill all the nodes when we are done, complain loudly
		// if there is any trouble as it could leave rogue filecoin
		// process running locally than must be manually killed
		for _, n := range nodes {
			err := n.Kill(true)
			require.NoError(err)
		}
	}()

	// Connect all the nodes together
	var wg sync.WaitGroup
	wg.Add(numNodes)
	for i := 0; i < numNodes; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < numNodes; j++ {
				err := iptb.ConnectNodes(nodes[i], nodes[j])
				require.NoError(err)
			}
		}(i)
	}
	wg.Wait()

	// Okay now we are ready to actually test stuff
	// everybody mine a single block, pausing for a moment to let the
	// mined block propagate to all the nodes
	for _, n := range nodes {
		// TODO make assertions on result of "mining once"
		_, err := n.RunCmd("go-filecoin", "mining", "once")
		require.NoError(err)

		// TODO replace sleep with something like a "message wait", as is
		// done in *_daemon tests. This will make the the test flaky
		time.Sleep(80 * time.Millisecond)
	}

	// get every nodes chain, validate their lengths
	nodeChains := make(map[string][]types.Block)
	for _, n := range nodes {
		// get the chain from the node as json
		out, err := n.RunCmd("go-filecoin", "chain", "ls", "--enc=json")
		require.NoError(err)

		// unmarshal the json string into a "list of blocks"
		chain, err := unmarshalChain(out)
		require.NoError(err)

		// minimum validation of chain should be numNodes + genesis
		require.True((len(chain) == numNodes+1))

		nodeChains[n.String()] = chain
	}

	// ensure all the chains are equivalent
	validateChains(t, nodeChains)
}

// unmarshalChain accepts an ndjson string and unmarshals it to an array of blocks.
func unmarshalChain(input string) ([]types.Block, error) {
	chain := strings.Trim(input, "\n")
	var bs []types.Block
	for _, line := range bytes.Split([]byte(chain), []byte{'\n'}) {
		var b types.Block
		err := json.Unmarshal(line, &b)
		if err != nil {
			return nil, err
		}
		bs = append(bs, b)
	}
	return bs, nil

}

// validateChains accepts a mapping of nodes to their chains. It compares
// every nodes chain with every other nodes chain. If they are not
// equivalent validateChains fails the calling test.
func validateChains(t *testing.T, nc map[string][]types.Block) {
	var wg sync.WaitGroup
	wg.Add(len(nc))

	// Grab a nodes chain
	for n := range nc {
		aChain := nc[n]
		// compare it with every other nodes chain
		go func(chain []types.Block) {
			defer wg.Done()
			for m := range nc {
				bChain := nc[m]
				chainsMatch(t, aChain, bChain)
			}
		}(aChain)
	}
}

// chainsMatch accepts 2 chains `A` and `B`. It compares their heads, if they
// are not equal chainsMatch fails the calling test.
func chainsMatch(t *testing.T, A, B []types.Block) {
	require.True(t, len(A) == len(B))
	require.True(t, A[0].Equals(&B[0]))
}
