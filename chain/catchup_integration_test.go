package chain_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestSimpleCaughtUpSync(t *testing.T) {
	tf.IntegrationTest(t)

	// Create a fake clock, advancing the time on this clock will cause the catchup execution loop
	// to run.
	fclk := th.NewFakeSystemClock(time.Unix(123456789, 0))
	_, nodes := makeNodes(t, 2, fclk)
	nd0 := nodes[0]
	nd1 := nodes[1]

	node.StartNodes(t, nodes)
	defer node.StopNodes(nodes)

	// catchup edgecase: if you are the first node on the network it is impossible to tell if you
	// are caught up. This handles that case by causing the genesis node to exit catchup mode.
	exitCaughtUpMode(t, fclk, nd0)

	mustConnect(t, nd0, nd1)

	// node1 will need to trust node0 to catchup from it.
	nd1.PeerTracker.Trust(nd0.Host().ID())

	// Mine a block with height == UntrustedChainHeightLimit
	nextBlk := requireMineOnce(context.Background(), t, nd0, chain.UntrustedChainHeightLimit)
	require.NoError(t, nd0.AddNewBlock(context.Background(), nextBlk))

	// first execution will catchup to node0
	executeCatchUpRoutine(t, fclk, nd1)
	// sececond execution will exit the catchup loop as we are now caught up.
	executeCatchUpRoutine(t, fclk, nd1)

	// ensure we catchup in a reasonable amout of time.
	syncTimeout := time.After(time.Second * 5)
	syncDone := make(chan struct{})
	go func() {
		nd1.ChainSynced.Wait()
		syncDone <- struct{}{}
	}()
	select {
	case <-syncTimeout:
		t.Fatal("timeout waiting for chain catchup")
	case <-syncDone:
	}

	maybeHeadTs := types.RequireNewTipSet(t, nextBlk)
	// verify all nodes are on the expected head
	nd0Status := nd0.Syncer.Status()
	nd1Status := nd1.Syncer.Status()
	assert.Equal(t, maybeHeadTs.Key().String(), nd0Status.ValidatedHead.String())
	assert.Equal(t, maybeHeadTs.Key().String(), nd1Status.ValidatedHead.String())
}

func TestMultiRoundCaughtUpSync(t *testing.T) {
	tf.IntegrationTest(t)

	// Create a fake clock, advancing the time on this clock will cause the catchup execution loop
	// to run.
	fclk := th.NewFakeSystemClock(time.Unix(123456789, 0))
	_, nodes := makeNodes(t, 2, fclk)
	nd0 := nodes[0]
	nd1 := nodes[1]

	node.StartNodes(t, nodes)
	defer node.StopNodes(nodes)

	// catchup edgecase: if you are the first node on the network it is impossible to tell if you
	// are caught up. This handles that case by causing the genesis node to exit catchup mode.
	exitCaughtUpMode(t, fclk, nd0)

	mustConnect(t, nd0, nd1)

	// node1 will need to trust node0 to catchup from it.
	nd1.PeerTracker.Trust(nd0.Host().ID())

	// Mine a block with height == UntrustedChainHeightLimit
	nextBlk := requireMineOnce(context.Background(), t, nd0, chain.UntrustedChainHeightLimit)
	require.NoError(t, nd0.AddNewBlock(context.Background(), nextBlk))

	// first execution will catchup to node0
	executeCatchUpRoutine(t, fclk, nd1)

	// add UntrustedChainHeightLimit blocks to node0, node1 is still not caught up
	nextBlk = requireMineOnce(context.Background(), t, nd0, chain.UntrustedChainHeightLimit)
	require.NoError(t, nd0.AddNewBlock(context.Background(), nextBlk))

	// sececond execution will catchup to node0 again
	executeCatchUpRoutine(t, fclk, nd1)

	// add UntrustedChainHeightLimit blocks to node0, node1 is still not caught up
	nextBlk = requireMineOnce(context.Background(), t, nd0, (chain.UntrustedChainHeightLimit/2)-3)
	require.NoError(t, nd0.AddNewBlock(context.Background(), nextBlk))

	// third execution should see that we are close enough, sync our head to node0 and exit catchup loop
	executeCatchUpRoutine(t, fclk, nd1)

	// ensure we catchup in a reasonable amout of time.
	syncTimeout := time.After(time.Second * 15)
	syncDone := make(chan struct{})
	go func() {
		nd1.ChainSynced.Wait()
		syncDone <- struct{}{}
	}()
	select {
	case <-syncTimeout:
		t.Fatal("timeout waiting for chain catchup")
	case <-syncDone:
	}

	maybeHeadTs := types.RequireNewTipSet(t, nextBlk)
	// verify all nodes are on the expected head
	nd0Status := nd0.Syncer.Status()
	nd1Status := nd1.Syncer.Status()
	assert.Equal(t, maybeHeadTs.Key().String(), nd0Status.ValidatedHead.String(), nd0Status.String())
	assert.Equal(t, maybeHeadTs.Key().String(), nd1Status.ValidatedHead.String())
}

// executeCatchUpRoutine will execute one round of catchup for all nodes using clock `fnc`.
func executeCatchUpRoutine(t *testing.T, fnc th.FakeSystemClock, nd *node.Node) {
	raw := nd.Repo.Config().Sync.CatchupSyncerPeriod
	cup, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatal(err)
	}
	fnc.Advance(cup)
}

// exitCaughtUpMode will cause node `nd` to exit caughtup mode.
func exitCaughtUpMode(t *testing.T, fnc th.FakeSystemClock, nd *node.Node) {
	raw := nd.Repo.Config().Sync.CatchupSyncerPeriod
	cup, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatal(err)
	}
	nd.CatchupSyncer.Stop()
	fnc.Advance(cup * 2)
}

func requireMineOnce(ctx context.Context, t *testing.T, minerNode *node.Node, numNulBlks int) *types.Block {
	head := minerNode.ChainReader.GetHead()
	headTipSet, err := minerNode.ChainReader.GetTipSet(head)
	require.NoError(t, err)
	baseTS := headTipSet
	require.NotNil(t, baseTS)

	worker, err := minerNode.CreateMiningWorker(ctx)
	require.NoError(t, err)

	// Miner should win first election as it has all the power so only
	// mine once with 0 null blocks
	out := make(chan mining.Output)
	var wonElection bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wonElection = worker.Mine(ctx, headTipSet, numNulBlks, out)
		wg.Done()
	}()
	next := <-out
	wg.Wait() // wait for wonElection to be set
	assert.True(t, wonElection)
	assert.NoError(t, next.Err)

	return next.NewBlock
}

func mustConnect(t *testing.T, nd1, nd2 *node.Node) {
	t.Helper()
	pinfo := peer.AddrInfo{
		ID:    nd2.Host().ID(),
		Addrs: nd2.Host().Addrs(),
	}

	if err := nd1.Host().Connect(context.Background(), pinfo); err != nil {
		require.NoError(t, err)
	}
}

// makeNodes makes at least two nodes, a miner and a client; numNodes is the total wanted, each node will
// be configured to use `clock` for timing.
func makeNodes(t *testing.T, numNodes int, clock th.FakeSystemClock) (address.Address, []*node.Node) {
	seed := node.MakeChainSeed(t, node.TestGenCfg)
	builderOpts := []node.BuilderOpt{node.ClockConfigOption(clock)}
	minerNode := node.MakeNodeWithChainSeed(t, seed, builderOpts,
		node.PeerKeyOpt(node.PeerKeys[0]),
	)
	seed.GiveKey(t, minerNode, 0)
	mineraddr, ownerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(mineraddr, ownerAddr, &storage.FakeProver{}, types.OneKiBSectorSize, minerNode, minerNode.Repo.DealsDatastore(), minerNode.PorcelainAPI)
	assert.NoError(t, err)

	nodes := []*node.Node{minerNode}

	nodeLimit := 1
	if numNodes > 2 {
		nodeLimit = numNodes
	}
	for i := 0; i < nodeLimit; i++ {
		nodes = append(nodes, node.MakeNodeWithChainSeed(t, seed, builderOpts))
	}
	return mineraddr, nodes
}
