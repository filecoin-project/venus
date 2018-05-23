package bigtest

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"sort"
	"strings"
	"sync"
	"time"

	iptb "gx/ipfs/QmYMTCRFe7Xgw2v47vqcxwDCPvkqafvdEbrZ2fFGK7u2VR/iptb/util"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/types"
)

var bigtest = flag.Bool("bigtest", false, "used to run the big tests") // nolint: deadcode

// MustStartIPTB initializes and starts `numNodes`.
func MustStartIPTB(r *require.Assertions, numNodes int, nodeType, deployType string, args ...string) []iptb.TestbedNode { // nolint: deadcode
	// initialize numNodes filecoin nodes locally and start them
	cfg := &iptb.InitCfg{
		Count:      numNodes,
		NodeType:   nodeType,
		DeployType: deployType,
		Force:      true,
		Bootstrap:  "skip",
	}
	r.NoError(iptb.TestbedInit(cfg))

	nodes, err := iptb.LoadNodes()
	r.NoError(err)

	r.NoError(iptb.TestbedStart(nodes, false, args))

	return nodes
}

// MustKillIPTB kills all nodes in `nodes`
func MustKillIPTB(r *require.Assertions, nodes []iptb.TestbedNode) {
	for _, n := range nodes {
		r.NoError(n.Kill(true))
	}
}

// MustRunIPTB runs command `cmd` on node `n`
func MustRunIPTB(r *require.Assertions, n iptb.TestbedNode, cmd ...string) string {
	out, err := n.RunCmd(cmd...)
	r.NoError(err)
	return out
}

// MustConnectIPTB will connect every node in `nodes` to each other.
// If a connecting fails, MustConnectIPTB fails the calling test.
func MustConnectIPTB(r *require.Assertions, nodes []iptb.TestbedNode) { // nolint: deadcode
	var wg sync.WaitGroup
	wg.Add(len(nodes))

	for i := 0; i < len(nodes); i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < len(nodes); j++ {
				r.NoError(iptb.ConnectNodes(nodes[i], nodes[j]))
			}
		}(i)
	}
	wg.Wait()
}

// MustHaveConditionBy requires every nodes in `nodes` to meet some condition
// defined in the function `cfn` within the duration `by`.
// The `cfn` accepts a slice of nodes to assert some condition on and a
// waitgroup for synchronization of node state.
func MustHaveConditionBy(r *require.Assertions, nodes []iptb.TestbedNode, wait time.Duration, cfn func(ctx context.Context, node iptb.TestbedNode, wg *sync.WaitGroup)) {
	done := make(chan struct{})
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, n := range nodes {
		wg.Add(1)
		go cfn(ctx, n, &wg)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return
	case <-time.After(wait):
		// Kill the remaining goroutines.
		r.FailNow("Timeout waiting for node state to sync")
	}
}

// MustHaveChainLengthBy requires every node in `nodes` to have a chain of length `size`
// by a duration `wait`.
func MustHaveChainLengthBy(r *require.Assertions, size int, wait time.Duration, nodes []iptb.TestbedNode) { // nolint: deadcode
	MustHaveConditionBy(r, nodes, wait, func(ctx context.Context, n iptb.TestbedNode, wg *sync.WaitGroup) {
		for {
			out := MustRunIPTB(r, n, "go-filecoin", "chain", "ls", "--enc=json")
			bc := MustUnmarshalChain(r, out)
			if len(bc) == size {
				wg.Done()
				return
			}
			if ctx.Err() != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

}

// MustHaveCidInPoolBy requires every node in `nodes` to have a message with cid `cid`
// in their message pool by duration `wait`.
func MustHaveCidInPoolBy(r *require.Assertions, cid string, wait time.Duration, nodes []iptb.TestbedNode) { // nolint: deadcode
	MustHaveConditionBy(r, nodes, wait, func(ctx context.Context, n iptb.TestbedNode, wg *sync.WaitGroup) {
		for {
			out := MustRunIPTB(r, n, "go-filecoin", "mpool")
			if strings.Contains(out, cid) {
				wg.Done()
				return
			}
			if ctx.Err() != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

// MustUnmarshalChain accepts an ndjson string and unmarshal it to a slice of blocks.
func MustUnmarshalChain(r *require.Assertions, input string) []types.Block { // nolint: deadcode
	chain := strings.Trim(input, "\n")
	var bs []types.Block

	for _, line := range bytes.Split([]byte(chain), []byte{'\n'}) {
		var b types.Block
		r.NoError(json.Unmarshal(line, &b))
		bs = append(bs, b)
	}

	return bs
}

// MustUnmarshalMessages accepts a string representing a JSON array and unmarshal it
// to a slice of messages
func MustUnmarshalMessages(r *require.Assertions, input string) []types.Message { // nolint: deadcode
	pool := strings.Trim(input, "\n")
	var ms []types.Message
	r.NoError(json.Unmarshal([]byte(pool), &ms))
	return ms
}

// MustGetCid gets the cid from message `msg` or panics
func MustGetCid(msg types.Message) *cid.Cid {
	cid, err := msg.Cid()
	if err != nil {
		panic(err)
	}
	return cid
}

// MustHaveEqualChainHeads takes an expected head block and a mapping of nodes
// to chains. It asserts that the head of each chain is equal to the expected head.
func MustHaveEqualChainHeads(a *assert.Assertions, expected types.Block, nodeChains map[int][]types.Block) { // nolint: deadcode
	for _, c := range nodeChains {
		actual := &c[0]
		a.True(expected.Equals(actual), "expected chain head: %+v, actual chain head: %+v", expected, actual)
	}
}

// MustHaveEqualMessages will first sort (mutate) `expected` and `nodeMsgs` by
// their cid's, then it will assert equality of each message cid in `expected`
// with each message cid in `nodeMsgs`.
func MustHaveEqualMessages(a *assert.Assertions, expected []types.Message, nodeMsgs map[int][]types.Message) { // nolint: deadcode
	expCids := MustExtractCids(expected)
	sort.Slice(expCids, func(i, j int) bool {
		return expCids[i].String() < expCids[j].String()
	})

	for _, actual := range nodeMsgs {
		// should have equal number of messages
		actCids := MustExtractCids(actual)
		sort.Slice(actCids, func(i, j int) bool {
			return actCids[i].String() < actCids[j].String()
		})
		a.True(len(actCids) == len(expCids), "expected length: %d, actual length: %d", len(expCids), len(actCids))
		for c := range actCids {
			a.True(expCids[c].Equals(actCids[c]), "expected cid: %s, actual cid: %s", expCids[c].String(), actCids[c].String())
		}
	}
}

// MustExtractCids will extract all cids from `msgs` and return them as a slice
func MustExtractCids(msgs []types.Message) []*cid.Cid {
	var cids []*cid.Cid
	for _, m := range msgs {
		cids = append(cids, MustGetCid(m))
	}
	return cids
}
