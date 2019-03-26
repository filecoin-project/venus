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
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/plumbing"
	pbConfig "github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/mthdsig"
	"github.com/filecoin-project/go-filecoin/plumbing/strgdls"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mockSigner, _ = types.NewMockSignersAndKeyInfo(10)

func TestNodeConstruct(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	nd := node.MakeNodesUnstarted(t, 1, false)[0]
	assert.NotNil(nd.Host)

	nd.Stop(context.Background())
}

func TestNodeNetworking(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert := assert.New(t)

	nds := node.MakeNodesUnstarted(t, 2, false)
	nd1, nd2 := nds[0], nds[1]

	pinfo := peerstore.PeerInfo{
		ID:    nd2.Host().ID(),
		Addrs: nd2.Host().Addrs(),
	}

	err := nd1.Host().Connect(ctx, pinfo)
	assert.NoError(err)

	nd1.Stop(ctx)
	nd2.Stop(ctx)
}

func TestConnectsToBootstrapNodes(t *testing.T) {
	t.Parallel()

	t.Run("no bootstrap nodes no problem", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		r := repo.NewInMemoryRepo()
		r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

		require.NoError(node.Init(ctx, r, consensus.DefaultGenesis))
		r.Config().Bootstrap.Addresses = []string{}
		opts, err := node.OptionsFromRepo(r)
		require.NoError(err)

		nd, err := node.New(ctx, opts...)
		require.NoError(err)
		assert.NoError(nd.Start(ctx))
		defer nd.Stop(ctx)
	})

	t.Run("connects to bootstrap nodes", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
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

		require.NoError(node.Init(ctx, r, consensus.DefaultGenesis))
		r.Config().Bootstrap.Addresses = []string{peer1, peer2}
		opts, err := node.OptionsFromRepo(r)
		require.NoError(err)
		nd, err := node.New(ctx, opts...)
		require.NoError(err)
		nd.Bootstrapper.MinPeerThreshold = 2
		nd.Bootstrapper.Period = 10 * time.Millisecond
		assert.NoError(nd.Start(ctx))
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

		assert.True(connected, "failed to connect")
	})
}

func TestNodeInit(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	ctx := context.Background()

	nd := node.MakeOfflineNode(t)

	assert.NoError(nd.Start(ctx))

	assert.NotNil(nd.ChainReader.GetHead())
	nd.Stop(ctx)
}

func TestNodeStartMining(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	ctx := context.Background()

	seed := node.MakeChainSeed(t, node.TestGenCfg)
	minerNode := node.MakeNodeWithChainSeed(t, seed, []node.ConfigOpt{}, node.PeerKeyOpt(node.PeerKeys[0]), node.AutoSealIntervalSecondsOpt(1))

	walletBackend, _ := wallet.NewDSBackend(minerNode.Repo.WalletDatastore())
	validator := consensus.NewDefaultMessageValidator()

	// TODO we need a principled way to construct an API that can be used both by node and by
	// tests. It should enable selective replacement of dependencies.
	plumbingAPI := plumbing.New(&plumbing.APIDeps{
		Chain:        minerNode.ChainReader,
		Config:       pbConfig.NewConfig(minerNode.Repo),
		MsgPool:      nil,
		MsgPreviewer: msg.NewPreviewer(minerNode.Wallet, minerNode.ChainReader, minerNode.CborStore(), minerNode.Blockstore),
		MsgQueryer:   msg.NewQueryer(minerNode.Repo, minerNode.Wallet, minerNode.ChainReader, minerNode.CborStore(), minerNode.Blockstore),
		MsgSender:    msg.NewSender(minerNode.Wallet, nil, nil, minerNode.Outbox, minerNode.MsgPool, validator, minerNode.PorcelainAPI.PubSubPublish),
		MsgWaiter:    msg.NewWaiter(minerNode.ChainReader, minerNode.Blockstore, minerNode.CborStore()),
		Network:      net.New(minerNode.Host(), nil, nil, nil, nil, nil),
		SigGetter:    mthdsig.NewGetter(minerNode.ChainReader),
		Wallet:       wallet.New(walletBackend),
		Deals:        strgdls.New(minerNode.Repo.DealsDatastore()),
	})
	porcelainAPI := porcelain.New(plumbingAPI)

	seed.GiveKey(t, minerNode, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(mineraddr, minerOwnerAddr, minerNode, minerNode.Repo.DealsDatastore(), porcelainAPI)
	assert.NoError(err)

	assert.NoError(minerNode.Start(ctx))

	t.Run("Start/Stop/Start results in a MiningScheduler that is started", func(t *testing.T) {
		assert.NoError(minerNode.StartMining(ctx))
		defer minerNode.StopMining(ctx)
		assert.True(minerNode.MiningScheduler.IsStarted())
		minerNode.StopMining(ctx)
		assert.False(minerNode.MiningScheduler.IsStarted())
		assert.NoError(minerNode.StartMining(ctx))
		assert.True(minerNode.MiningScheduler.IsStarted())
	})

	t.Run("Start + Start gives an error message saying mining is already started", func(t *testing.T) {
		assert.NoError(minerNode.StartMining(ctx))
		defer minerNode.StopMining(ctx)
		err := minerNode.StartMining(ctx)
		assert.Error(err, "node is already mining")
	})

}

func TestUpdateMessagePool(t *testing.T) {
	t.Parallel()
	// Note: majority of tests are in message_pool_test. This test
	// just makes sure it looks like it is hooked up correctly.
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	node := node.MakeNodesUnstarted(t, 1, true)[0]
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(ok)

	// Msg pool: [m0, m1],   Chain: gen -> b[m2, m3]
	// to
	// Msg pool: [m0, m3],   Chain: gen -> b[] -> b[m1, m2]
	assert.NoError(chainForTest.Load(ctx)) // load up head to get genesis block
	head := chainForTest.GetHead()
	headTipSetAndState, err := chainForTest.GetTipSetAndState(ctx, head)
	require.NoError(err)
	genTS := headTipSetAndState.TipSet
	m := types.NewSignedMsgs(4, mockSigner)
	core.MustAdd(node.MsgPool, m[0], m[1])

	oldChain := core.NewChainWithMessages(node.CborStore(), genTS, [][]*types.SignedMessage{{m[2], m[3]}})
	newChain := core.NewChainWithMessages(node.CborStore(), genTS, [][]*types.SignedMessage{{}}, [][]*types.SignedMessage{{m[1], m[2]}})

	th.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          oldChain[len(oldChain)-1],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	assert.NoError(chainForTest.SetHead(ctx, oldChain[len(oldChain)-1]))
	assert.NoError(node.Start(ctx))
	updateMsgPoolDoneCh := make(chan struct{})
	node.HeaviestTipSetHandled = func() { updateMsgPoolDoneCh <- struct{}{} }
	// Triggers a notification, node should update the message pool as a result.
	th.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          newChain[len(newChain)-2],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	th.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          newChain[len(newChain)-1],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	assert.NoError(chainForTest.SetHead(ctx, newChain[len(newChain)-1]))
	<-updateMsgPoolDoneCh
	assert.Equal(2, len(node.MsgPool.Pending()))
	pending := node.MsgPool.Pending()

	assert.True(types.SmsgCidsEqual(m[0], pending[0]) || types.SmsgCidsEqual(m[0], pending[1]))
	assert.True(types.SmsgCidsEqual(m[3], pending[0]) || types.SmsgCidsEqual(m[3], pending[1]))
	node.Stop(ctx)
}

func TestOptionWithError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert := assert.New(t)
	r := repo.NewInMemoryRepo()
	assert.NoError(node.Init(ctx, r, consensus.DefaultGenesis))

	opts, err := node.OptionsFromRepo(r)
	assert.NoError(err)

	scaryErr := errors.New("i am an error grrrr")
	errOpt := func(c *node.Config) error {
		return scaryErr
	}

	opts = append(opts, errOpt)

	_, err = node.New(ctx, opts...)
	assert.Error(err, scaryErr)

}

func TestNodeConfig(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

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

	assert.Equal(actualBlockTime, blockTime)
	assert.Equal(true, n.OfflineMode)
	assert.Equal(defaultCfg.Mining, cfg.Mining)
	assert.Equal(&config.SwarmConfig{
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
