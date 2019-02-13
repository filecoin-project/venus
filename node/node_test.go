package node

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/plumbing"
	pbConfig "github.com/filecoin-project/go-filecoin/plumbing/cfg"
	"github.com/filecoin-project/go-filecoin/plumbing/chn"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/plumbing/mthdsig"
	"github.com/filecoin-project/go-filecoin/plumbing/ntwk"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)

func TestNodeConstruct(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	nd := MakeNodesUnstarted(t, 1, false, true)[0]
	assert.NotNil(nd.Host)

	nd.Stop(context.Background())
}

func TestNodeNetworking(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert := assert.New(t)

	nds := MakeNodesUnstarted(t, 2, false, true)
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

		require.NoError(Init(ctx, r, consensus.InitGenesis))
		r.Config().Bootstrap.Addresses = []string{}
		opts, err := OptionsFromRepo(r)
		require.NoError(err)

		nd, err := New(ctx, opts...)
		require.NoError(err)
		assert.NoError(nd.Start(ctx))
		defer nd.Stop(ctx)
	})

	t.Run("connects to bootstrap nodes", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		// These are two bootstrap nodes we'll connect to.
		nds := MakeNodesStarted(t, 2, false, true)
		nd1, nd2 := nds[0], nds[1]

		// Gotta be a better way to do this?
		peer1 := fmt.Sprintf("%s/ipfs/%s", nd1.Host().Addrs()[0].String(), nd1.Host().ID().Pretty())
		peer2 := fmt.Sprintf("%s/ipfs/%s", nd2.Host().Addrs()[0].String(), nd2.Host().ID().Pretty())

		// Create a node with the nodes above as bootstrap nodes.
		r := repo.NewInMemoryRepo()
		r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

		require.NoError(Init(ctx, r, consensus.InitGenesis))
		r.Config().Bootstrap.Addresses = []string{peer1, peer2}
		opts, err := OptionsFromRepo(r)
		require.NoError(err)
		nd, err := New(ctx, opts...)
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

	nd := MakeOfflineNode(t)

	assert.NoError(nd.Start(ctx))

	assert.NotNil(nd.ChainReader.Head())
	nd.Stop(ctx)
}

func TestNodeStartMining(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	//require := require.New(t)
	ctx := context.Background()

	seed := MakeChainSeed(t, TestGenCfg)
	minerNode := MakeNodeWithChainSeed(t, seed, []ConfigOpt{}, PeerKeyOpt(PeerKeys[0]), AutoSealIntervalSecondsOpt(1))

	walletBackend, _ := wallet.NewDSBackend(minerNode.Repo.WalletDatastore())

	// TODO we need a principled way to construct an API that can be used both by node and by
	// tests. It should enable selective replacement of dependencies.
	plumbingAPI := plumbing.New(&plumbing.APIDeps{
		SigGetter:    mthdsig.NewGetter(minerNode.ChainReader),
		MsgPreviewer: msg.NewPreviewer(minerNode.Wallet, minerNode.ChainReader, minerNode.CborStore(), minerNode.Blockstore),
		MsgQueryer:   msg.NewQueryer(minerNode.Repo, minerNode.Wallet, minerNode.ChainReader, minerNode.CborStore(), minerNode.Blockstore),
		MsgSender:    msg.NewSender(minerNode.Repo, minerNode.Wallet, minerNode.ChainReader, minerNode.MsgPool, minerNode.PubSub.Publish),
		MsgWaiter:    msg.NewWaiter(minerNode.ChainReader, minerNode.Blockstore, minerNode.CborStore()),
		Config:       pbConfig.NewConfig(minerNode.Repo),
		Chain:        chn.New(minerNode.ChainReader),
		Network:      ntwk.NewNetwork(minerNode.Host()),
		Wallet:       wallet.New(walletBackend),
	})
	porcelainAPI := porcelain.New(plumbingAPI)

	seed.GiveKey(t, minerNode, 0)
	mineraddr, minerOwnerAddr := seed.GiveMiner(t, minerNode, 0)
	_, err := storage.NewMiner(ctx, mineraddr, minerOwnerAddr, minerNode, minerNode.Repo.DealsDatastore(), porcelainAPI)
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

// skipped anyway, now commented out.  With new mining we really need something here though.
/*
func TestNodeMining(t *testing.T) {
	t.Skip("Bad Test, stop messing with __all__ internals of the node, write a better test!")
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	newCid := types.NewCidForTestGetter()
	ctx := context.Background()

	node := MakeNodeUnstartedSeed(t, true, true)

	mockScheduler := &mining.MockScheduler{}
	outCh, doneWg := make(chan mining.Output), new(sync.WaitGroup)
	// Apparently you have to have exact types for testify.mock, so
	// we use iCh and oCh for the specific return type of Start().
	var oCh <-chan mining.Output = outCh

	mockScheduler.On("Start", mock.Anything).Return(oCh, doneWg)
	node.MiningScheduler = mockScheduler
	// TODO: this is horrible, this setup needs to be a lot less dependent of the inner workings of the node!!
	node.miningCtx, node.cancelMining = context.WithCancel(ctx)
	node.miningDoneWg = doneWg
	go node.handleNewMiningOutput(oCh)

	// Ensure that the initial input (the best tipset) is wired up properly.
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(ok)
	require.NoError(node.Start(ctx))
	genTS := chainForTest.Head()
	b1 := genTS.ToSlice()[0]
	require.NoError(node.StartMining(ctx))

	// Ensure that the successive inputs (new best tipsets) are wired up properly.
	b2 := chain.RequireMkFakeChild(require, genTS, node.ChainReader.GenesisCid(), newCid(), uint64(0), uint64(0))
	chainForTest.SetHead(ctx, consensus.RequireNewTipSet(require, b2))

	node.StopMining(ctx)
	chainForTest.SetHead(ctx, consensus.RequireNewTipSet(require, b2))

	time.Sleep(20 * time.Millisecond)
	assert.Equal(mining.ChannelEmpty, mining.ReceiveInCh(inCh))

	// Ensure we're tearing down cleanly.
	// Part of stopping cleanly is waiting for the worker to be done.
	// Kinda lame to test this way, but better than not testing.
	node = MakeNodeUnstartedSeed(t, true, true)

	assert.NoError(node.Start(ctx))
	assert.NoError(node.StartMining(ctx))

	workerDone := false
	node.miningDoneWg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		workerDone = true
		node.miningDoneWg.Done()
	}()
	node.Stop(ctx)
	assert.True(workerDone)

	// Ensure that the output is wired up correctly.
	node = MakeNodeUnstartedSeed(t, true, true)

	mockScheduler = &mining.MockScheduler{}
	inCh, outCh, doneWg = make(chan mining.Input), make(chan mining.Output), new(sync.WaitGroup)
	iCh = inCh
	oCh = outCh
	mockScheduler.On("Start", mock.Anything).Return(iCh, oCh, doneWg)
	node.MiningScheduler = mockScheduler
	node.miningCtx, node.cancelMining = context.WithCancel(ctx)
	node.miningInCh = inCh
	node.miningDoneWg = doneWg
	go node.handleNewMiningOutput(oCh)
	assert.NoError(node.Start(ctx))

	var gotBlock *types.Block
	gotBlockCh := make(chan struct{})
	node.AddNewlyMinedBlock = func(ctx context.Context, b *types.Block) {
		gotBlock = b
		go func() { gotBlockCh <- struct{}{} }()
	}
	assert.NoError(node.StartMining(ctx))
	go func() { outCh <- mining.NewOutput(b1, nil) }()
	<-gotBlockCh
	assert.True(b1.Cid().Equals(gotBlock.Cid()))
}*/

func TestUpdateMessagePool(t *testing.T) {
	t.Parallel()
	// Note: majority of tests are in message_pool_test. This test
	// just makes sure it looks like it is hooked up correctly.
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	node := MakeNodesUnstarted(t, 1, true, false)[0]
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(ok)

	// Msg pool: [m0, m1],   Chain: gen -> b[m2, m3]
	// to
	// Msg pool: [m0, m3],   Chain: gen -> b[] -> b[m1, m2]
	assert.NoError(chainForTest.Load(ctx)) // load up head to get genesis block
	genTS := chainForTest.Head()
	m := types.NewSignedMsgs(4, mockSigner)
	core.MustAdd(node.MsgPool, m[0], m[1])

	oldChain := core.NewChainWithMessages(node.CborStore(), genTS, [][]*types.SignedMessage{{m[2], m[3]}})
	newChain := core.NewChainWithMessages(node.CborStore(), genTS, [][]*types.SignedMessage{{}}, [][]*types.SignedMessage{{m[1], m[2]}})

	chain.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          oldChain[len(oldChain)-1],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	assert.NoError(chainForTest.SetHead(ctx, oldChain[len(oldChain)-1]))
	assert.NoError(node.Start(ctx))
	updateMsgPoolDoneCh := make(chan struct{})
	node.HeaviestTipSetHandled = func() { updateMsgPoolDoneCh <- struct{}{} }
	// Triggers a notification, node should update the message pool as a result.
	chain.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          newChain[len(newChain)-2],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	chain.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
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
	assert.NoError(Init(ctx, r, consensus.InitGenesis))

	opts, err := OptionsFromRepo(r)
	assert.NoError(err)

	scaryErr := errors.New("i am an error grrrr")
	errOpt := func(c *Config) error {
		return scaryErr
	}

	opts = append(opts, errOpt)

	_, err = New(ctx, opts...)
	assert.Error(err, scaryErr)

}

func TestMakePrivateKey(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// should fail if less than 1024
	badKey, err := makePrivateKey(10)
	assert.Error(err, ErrLittleBits)
	assert.Nil(badKey)

	// 1024 should work
	okKey, err := makePrivateKey(1024)
	assert.NoError(err)
	assert.NotNil(okKey)

	// large values should work
	goodKey, err := makePrivateKey(4096)
	assert.NoError(err)
	assert.NotNil(goodKey)
}

func repoConfig() ConfigOpt {
	defaultCfg := config.NewDefaultConfig()
	return func(c *Config) error {
		// overwrite value set with testhelpers.GetFreePort()
		c.Repo.Config().API.Address = defaultCfg.API.Address
		return nil
	}
}

func TestNodeConfig(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	defaultCfg := config.NewDefaultConfig()

	// fake mining/always a winning ticket
	verifier := proofs.NewFakeVerifier(true, nil)

	configBlockTime := 99

	configOptions := []ConfigOpt{
		repoConfig(),
		VerifierConfigOption(verifier),
		BlockTime(time.Duration(configBlockTime)),
	}

	initOpts := []InitOpt{AutoSealIntervalSecondsOpt(120)}

	tno := TestNodeOptions{
		ConfigOpts:  configOptions,
		InitOpts:    initOpts,
		OfflineMode: true,
		GenesisFunc: consensus.InitGenesis,
	}

	n := GenNode(t, &tno)
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
