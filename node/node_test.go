package node_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

func TestNodeConstruct(t *testing.T) {
	tf.UnitTest(t)

	nd := node.MakeNodesUnstarted(t, 1, false)[0]
	assert.NotNil(t, nd.Host)

	nd.Stop(context.Background())
}

func TestNodeNetworking(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	nds := node.MakeNodesUnstarted(t, 2, false)
	nd1, nd2 := nds[0], nds[1]

	pinfo := peerstore.PeerInfo{
		ID:    nd2.Host().ID(),
		Addrs: nd2.Host().Addrs(),
	}

	err := nd1.Host().Connect(ctx, pinfo)
	assert.NoError(t, err)

	nd1.Stop(ctx)
	nd2.Stop(ctx)
}

func TestConnectsToBootstrapNodes(t *testing.T) {
	tf.UnitTest(t)

	t.Run("no bootstrap nodes no problem", func(t *testing.T) {
		ctx := context.Background()

		r := repo.NewInMemoryRepo()
		r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

		require.NoError(t, node.Init(ctx, r, consensus.DefaultGenesis))
		r.Config().Bootstrap.Addresses = []string{}
		opts, err := node.OptionsFromRepo(r)
		require.NoError(t, err)

		nd, err := node.New(ctx, opts...)
		require.NoError(t, err)
		assert.NoError(t, nd.Start(ctx))
		defer nd.Stop(ctx)
	})

	t.Run("connects to bootstrap nodes", func(t *testing.T) {
		ctx := context.Background()

		// These are two bootstrap nodes we'll connect to.
		nds := node.MakeNodesUnstarted(t, 2, false)
		node.StartNodes(t, nds)
		nd1, nd2 := nds[0], nds[1]

		// Gotta be a better way to do this?
		peer1 := fmt.Sprintf("%s/ipfs/%s", nd1.Host().Addrs()[0].String(), nd1.Host().ID().Pretty())
		peer2 := fmt.Sprintf("%s/ipfs/%s", nd2.Host().Addrs()[0].String(), nd2.Host().ID().Pretty())

		// Create a node with the nodes above as bootstrap nodes.
		r := repo.NewInMemoryRepo()
		r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

		require.NoError(t, node.Init(ctx, r, consensus.DefaultGenesis))
		r.Config().Bootstrap.Addresses = []string{peer1, peer2}

		opts, err := node.OptionsFromRepo(r)
		require.NoError(t, err)
		nd, err := node.New(ctx, opts...)
		require.NoError(t, err)
		nd.Bootstrapper.MinPeerThreshold = 2
		nd.Bootstrapper.Period = 10 * time.Millisecond
		assert.NoError(t, nd.Start(ctx))
		defer nd.Stop(ctx)

		// Ensure they're connected.
		connected := false
		// poll until we are connected, to avoid flaky tests
		for i := 0; i <= 30; i++ {
			l1 := len(nd.Host().Network().ConnsToPeer(nd1.Host().ID()))
			l2 := len(nd.Host().Network().ConnsToPeer(nd2.Host().ID()))

			connected = l1 == 1 && l2 == 1
			if connected {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		assert.True(t, connected, "failed to connect")
	})
}

func TestNodeInit(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	nd := node.MakeOfflineNode(t)

	assert.NoError(t, nd.Start(ctx))

	assert.NotEqual(t, 0, nd.ChainReader.GetHead().Len())
	nd.Stop(ctx)
}

func TestNodeStartMining(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	seed := node.MakeChainSeed(t, node.TestGenCfg)
	minerNode := node.MakeNodeWithChainSeed(t, seed, []node.ConfigOpt{}, node.PeerKeyOpt(node.PeerKeys[0]), node.AutoSealIntervalSecondsOpt(1))

	seed.GiveKey(t, minerNode, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(mineraddr, minerOwnerAddr, minerNode, minerNode.Repo.DealsDatastore(), nil)
	assert.NoError(t, err)

	assert.NoError(t, minerNode.Start(ctx))

	t.Run("Start/Stop/Start results in a MiningScheduler that is started", func(t *testing.T) {
		assert.NoError(t, minerNode.StartMining(ctx))
		defer minerNode.StopMining(ctx)
		assert.True(t, minerNode.MiningScheduler.IsStarted())
		minerNode.StopMining(ctx)
		assert.False(t, minerNode.MiningScheduler.IsStarted())
		assert.NoError(t, minerNode.StartMining(ctx))
		assert.True(t, minerNode.MiningScheduler.IsStarted())
	})

	t.Run("Start + Start gives an error message saying mining is already started", func(t *testing.T) {
		assert.NoError(t, minerNode.StartMining(ctx))
		defer minerNode.StopMining(ctx)
		err := minerNode.StartMining(ctx)
		assert.Error(t, err, "node is already mining")
	})

}

func TestUpdateMessagePool(t *testing.T) {
	tf.UnitTest(t)

	// Note: majority of tests are in message_pool_test. This test
	// just makes sure it looks like it is hooked up correctly.
	ctx := context.Background()
	node := node.MakeNodesUnstarted(t, 1, true)[0]
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(t, ok)

	// Msg pool: [m0, m1],   Chain: gen -> b[m2, m3]
	// to
	// Msg pool: [m0, m3],   Chain: gen -> b[] -> b[m1, m2]
	assert.NoError(t, chainForTest.Load(ctx)) // load up head to get genesis block
	head := chainForTest.GetHead()
	headTipSet, err := chainForTest.GetTipSet(head)
	require.NoError(t, err)
	genTS := *headTipSet
	m := types.NewSignedMsgs(4, mockSigner)
	_, err = node.Inbox.Add(ctx, m[0])
	require.NoError(t, err)
	_, err = node.Inbox.Add(ctx, m[1])
	require.NoError(t, err)

	oldChain := core.NewChainWithMessages(node.CborStore(), genTS, [][]*types.SignedMessage{{m[2], m[3]}})
	newChain := core.NewChainWithMessages(node.CborStore(), genTS, [][]*types.SignedMessage{{}}, [][]*types.SignedMessage{{m[1], m[2]}})

	th.RequirePutTsas(ctx, t, chainForTest, &chain.TipSetAndState{
		TipSet:          oldChain[len(oldChain)-1],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	assert.NoError(t, chainForTest.SetHead(ctx, oldChain[len(oldChain)-1]))
	assert.NoError(t, node.Start(ctx))
	updateMsgPoolDoneCh := make(chan struct{})
	node.HeaviestTipSetHandled = func() { updateMsgPoolDoneCh <- struct{}{} }
	// Triggers a notification, node should update the message pool as a result.
	th.RequirePutTsas(ctx, t, chainForTest, &chain.TipSetAndState{
		TipSet:          newChain[len(newChain)-2],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	th.RequirePutTsas(ctx, t, chainForTest, &chain.TipSetAndState{
		TipSet:          newChain[len(newChain)-1],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	assert.NoError(t, chainForTest.SetHead(ctx, newChain[len(newChain)-1]))
	<-updateMsgPoolDoneCh
	assert.Equal(t, 2, len(node.Inbox.Pool().Pending()))
	pending := node.Inbox.Pool().Pending()

	assert.True(t, types.SmsgCidsEqual(m[0], pending[0]) || types.SmsgCidsEqual(m[0], pending[1]))
	assert.True(t, types.SmsgCidsEqual(m[3], pending[0]) || types.SmsgCidsEqual(m[3], pending[1]))
	node.Stop(ctx)
}

func TestOptionWithError(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	r := repo.NewInMemoryRepo()
	assert.NoError(t, node.Init(ctx, r, consensus.DefaultGenesis))

	opts, err := node.OptionsFromRepo(r)
	assert.NoError(t, err)

	scaryErr := errors.New("i am an error grrrr")
	errOpt := func(c *node.Config) error {
		return scaryErr
	}

	opts = append(opts, errOpt)

	_, err = node.New(ctx, opts...)
	assert.Error(t, err, scaryErr)

}

func TestNodeConfig(t *testing.T) {
	tf.UnitTest(t)

	defaultCfg := config.NewDefaultConfig()

	// fake mining
	verifier := proofs.NewFakeVerifier(true, nil)

	configBlockTime := 99

	configOptions := []node.ConfigOpt{
		repoConfig(),
		node.VerifierConfigOption(verifier),
		node.BlockTime(time.Duration(configBlockTime)),
	}

	initOpts := []node.InitOpt{node.AutoSealIntervalSecondsOpt(120)}

	tno := node.TestNodeOptions{
		ConfigOpts:  configOptions,
		InitOpts:    initOpts,
		OfflineMode: true,
		GenesisFunc: consensus.DefaultGenesis,
	}

	n := node.GenNode(t, &tno)
	cfg := n.Repo.Config()
	_, blockTime := n.MiningTimes()

	actualBlockTime := time.Duration(configBlockTime / mining.MineDelayConversionFactor)

	assert.Equal(t, actualBlockTime, blockTime)
	assert.Equal(t, true, n.OfflineMode)
	assert.Equal(t, defaultCfg.Mining, cfg.Mining)
	assert.Equal(t, &config.SwarmConfig{
		Address: "/ip4/0.0.0.0/tcp/0",
	}, cfg.Swarm)
}

func repoConfig() node.ConfigOpt {
	defaultCfg := config.NewDefaultConfig()
	return func(c *node.Config) error {
		// overwrite value set with th.GetFreePort()
		c.Repo.Config().API.Address = defaultCfg.API.Address
		return nil
	}
}
