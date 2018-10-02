package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmeKD8YT7887Xu6Z86iZmpYNxrLogJexqxEugSmaf14k64/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var seed = types.GenerateKeyInfoSeed()
var ki = types.MustGenerateKeyInfo(10, seed)
var mockSigner = types.NewMockSigner(ki)
var newSignedMessage = types.NewSignedMessageForTestGetter(mockSigner)

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
		ID:    nd2.Host.ID(),
		Addrs: nd2.Host.Addrs(),
	}

	err := nd1.Host.Connect(ctx, pinfo)
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
		peer1 := fmt.Sprintf("%s/ipfs/%s", nd1.Host.Addrs()[0].String(), nd1.Host.ID().Pretty())
		peer2 := fmt.Sprintf("%s/ipfs/%s", nd2.Host.Addrs()[0].String(), nd2.Host.ID().Pretty())

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
			l1 := len(nd.Host.Network().ConnsToPeer(nd1.Host.ID()))
			l2 := len(nd.Host.Network().ConnsToPeer(nd2.Host.ID()))

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

	nd := MakeNodesUnstarted(t, 1, true, true)[0]

	assert.NoError(nd.Start(ctx))

	assert.NotNil(nd.ChainReader.Head())
	nd.Stop(ctx)
}

func TestNodeMining(t *testing.T) {
	t.Skip("Bad Test, stop messing with __all__ internals of the node, write a better test!")
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	newCid := types.NewCidForTestGetter()
	ctx := context.Background()

	node := MakeNodeUnstartedSeed(t, true, true)

	mockScheduler := &mining.MockScheduler{}
	inCh, outCh, doneWg := make(chan mining.Input), make(chan mining.Output), new(sync.WaitGroup)
	// Apparently you have to have exact types for testify.mock, so
	// we use iCh and oCh for the specific return type of Start().
	var iCh chan<- mining.Input = inCh
	var oCh <-chan mining.Output = outCh

	mockScheduler.On("Start", mock.Anything).Return(iCh, oCh, doneWg)
	node.MiningScheduler = mockScheduler
	// TODO: this is horrible, this setup needs to be a lot less dependent of the inner workings of the node!!
	node.miningCtx, node.cancelMining = context.WithCancel(ctx)
	node.miningInCh = inCh
	node.miningDoneWg = doneWg
	go node.handleNewMiningOutput(oCh)

	// Ensure that the initial input (the best tipset) is wired up properly.
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(ok)
	require.NoError(node.Start(ctx))
	genTS := chainForTest.Head()
	b1 := genTS.ToSlice()[0]
	require.NoError(node.StartMining(ctx))
	gotInput := <-inCh
	require.Equal(1, len(gotInput.TipSet))
	assert.True(b1.Cid().Equals(gotInput.TipSet.ToSlice()[0].Cid()))

	// Ensure that the successive inputs (new best tipsets) are wired up properly.
	b2 := chain.RequireMkFakeChild(require, genTS, node.ChainReader.GenesisCid(), newCid(), uint64(0), uint64(0))
	chainForTest.SetHead(ctx, consensus.RequireNewTipSet(require, b2))
	gotInput = <-inCh
	require.Equal(1, len(gotInput.TipSet))
	assert.True(b2.Cid().Equals(gotInput.TipSet.ToSlice()[0].Cid()))

	// Ensure we don't mine when stopped.
	assert.Equal(mining.ChannelEmpty, mining.ReceiveInCh(inCh))

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
}

func TestUpdateMessagePool(t *testing.T) {
	t.Parallel()
	// Note: majority of tests are in message_pool_test. This test
	// just makes sure it looks like it is hooked up correctly.
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	node := MakeNodesUnstarted(t, 1, true, true)[0]
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(ok)

	// Msg pool: [m0, m1],   Chain: gen -> b[m2, m3]
	// to
	// Msg pool: [m0, m3],   Chain: gen -> b[] -> b[m1, m2]
	chainForTest.Load(ctx) // load up head to get genesis block
	genTS := chainForTest.Head()
	m := types.NewSignedMsgs(4, mockSigner)
	core.MustAdd(node.MsgPool, m[0], m[1])

	oldChain := core.NewChainWithMessages(node.CborStore, genTS, [][]*types.SignedMessage{{m[2], m[3]}})
	newChain := core.NewChainWithMessages(node.CborStore, genTS, [][]*types.SignedMessage{{}}, [][]*types.SignedMessage{{m[1], m[2]}})

	chain.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          oldChain[len(oldChain)-1],
		TipSetStateRoot: genTS.ToSlice()[0].StateRoot,
	})
	chainForTest.SetHead(ctx, oldChain[len(oldChain)-1])
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
	chainForTest.SetHead(ctx, newChain[len(newChain)-1])
	<-updateMsgPoolDoneCh
	assert.Equal(2, len(node.MsgPool.Pending()))
	pending := node.MsgPool.Pending()
	assert.True(types.SmsgCidsEqual(m[0], pending[0]) || types.SmsgCidsEqual(m[0], pending[1]))
	assert.True(types.SmsgCidsEqual(m[3], pending[0]) || types.SmsgCidsEqual(m[3], pending[1]))
	node.Stop(ctx)
}

func testWaitHelp(wg *sync.WaitGroup, assert *assert.Assertions, node *Node, expectMsg *types.SignedMessage, expectError bool, cb func(*types.Block, *types.SignedMessage, *types.MessageReceipt) error) {
	expectCid, err := expectMsg.Cid()
	if cb == nil {
		cb = func(b *types.Block, msg *types.SignedMessage,
			rcp *types.MessageReceipt) error {
			assert.True(types.SmsgCidsEqual(expectMsg, msg))
			if wg != nil {
				wg.Done()
			}

			return nil
		}
	}
	assert.NoError(err)

	err = node.WaitForMessage(context.Background(), expectCid, cb)
	assert.Equal(expectError, err != nil)
}

type smsgs []*types.SignedMessage
type smsgsSet [][]*types.SignedMessage

func TestWaitForMessage(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	node := MakeNodesUnstarted(t, 1, true, true)[0]

	err := node.Start(ctx)
	assert.NoError(err)

	testWaitExisting(ctx, assert, require, node)
	testWaitNew(ctx, assert, require, node)
}

func testWaitExisting(ctx context.Context, assert *assert.Assertions, require *require.Assertions, node *Node) {
	chainStore, ok := node.ChainReader.(chain.Store)
	assert.True(ok)

	m1, m2 := newSignedMessage(), newSignedMessage()
	chainWithMsgs := core.NewChainWithMessages(node.CborStore, node.ChainReader.Head(), smsgsSet{smsgs{m1, m2}})
	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(1, len(ts))
	chain.RequirePutTsas(ctx, require, chainStore, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].StateRoot,
	})
	require.NoError(chainStore.SetHead(ctx, ts))

	testWaitHelp(nil, assert, node, m1, false, nil)
	testWaitHelp(nil, assert, node, m2, false, nil)
}

func testWaitNew(ctx context.Context, assert *assert.Assertions, require *require.Assertions, node *Node) {
	var wg sync.WaitGroup
	chainStore, ok := node.ChainReader.(chain.Store)
	assert.True(ok)

	_, _ = newSignedMessage(), newSignedMessage() // flush out so we get distinct messages from testWaitExisting
	m3, m4 := newSignedMessage(), newSignedMessage()
	chainWithMsgs := core.NewChainWithMessages(node.CborStore, node.ChainReader.Head(), smsgsSet{smsgs{m3, m4}})

	wg.Add(2)
	go testWaitHelp(&wg, assert, node, m3, false, nil)
	go testWaitHelp(&wg, assert, node, m4, false, nil)
	time.Sleep(10 * time.Millisecond)

	ts := chainWithMsgs[len(chainWithMsgs)-1]
	require.Equal(1, len(ts))
	chain.RequirePutTsas(ctx, require, chainStore, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: ts.ToSlice()[0].StateRoot,
	})
	require.NoError(chainStore.SetHead(ctx, ts))

	wg.Wait()
}

func TestWaitForMessageError(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	node := MakeNodesUnstarted(t, 1, true, true)[0]

	err := node.Start(ctx)
	assert.NoError(err)

	testWaitError(ctx, assert, require, node)
}

func testWaitError(ctx context.Context, assert *assert.Assertions, require *require.Assertions, node *Node) {
	chainStore, ok := node.ChainReader.(chain.Store)
	assert.True(ok)

	m1, m2, m3, m4 := newSignedMessage(), newSignedMessage(), newSignedMessage(), newSignedMessage()
	chain := core.NewChainWithMessages(node.CborStore, node.ChainReader.Head(), smsgsSet{smsgs{m1, m2}}, smsgsSet{smsgs{m3, m4}})
	// set the head without putting the ancestor block in the chainStore.
	err := chainStore.SetHead(ctx, chain[len(chain)-1])
	assert.Nil(err)

	testWaitHelp(nil, assert, node, m2, true, nil)
}

func TestWaitConflicting(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	addr1, addr2, addr3 := mockSigner.Addresses[0], mockSigner.Addresses[1], mockSigner.Addresses[2]
	testGen := consensus.MakeGenesisFunc(
		consensus.ActorAccount(addr1, types.NewAttoFILFromFIL(10000)),
		consensus.ActorAccount(addr2, types.NewAttoFILFromFIL(0)),
		consensus.ActorAccount(addr3, types.NewAttoFILFromFIL(0)),
	)
	node := MakeNodesUnstartedWithGif(t, 1, true, true, testGen)[0]
	chainForTest, ok := node.ChainReader.(chain.Store)
	require.True(ok)

	assert.NoError(node.Start(ctx))

	// Create conflicting messages
	m1 := types.NewMessage(addr1, addr3, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm1, err := types.NewSignedMessage(*m1, &mockSigner)
	require.NoError(err)

	m2 := types.NewMessage(addr1, addr2, 0, types.NewAttoFILFromFIL(6000), "", nil)
	sm2, err := types.NewSignedMessage(*m2, &mockSigner)
	require.NoError(err)

	baseTS := node.ChainReader.Head()
	require.Equal(1, len(baseTS))
	baseBlock := baseTS.ToSlice()[0]

	b1 := chain.RequireMkFakeChild(require, baseTS, node.ChainReader.GenesisCid(), baseBlock.StateRoot, uint64(0), uint64(0))
	b1.Messages = []*types.SignedMessage{sm1}
	b1.Ticket = []byte{0} // block 1 comes first in message application
	core.MustPut(node.CborStore, b1)

	b2 := chain.RequireMkFakeChild(require, baseTS, node.ChainReader.GenesisCid(), baseBlock.StateRoot, uint64(1), uint64(0))
	b2.Messages = []*types.SignedMessage{sm2}
	b2.Ticket = []byte{1}
	core.MustPut(node.CborStore, b2)

	ts := consensus.RequireNewTipSet(require, b1, b2)
	chain.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
		TipSet:          ts,
		TipSetStateRoot: baseBlock.StateRoot,
	})
	chainForTest.SetHead(ctx, ts)
	msgApplySucc := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.NotNil(rcp)
		return nil
	}
	msgApplyFail := func(b *types.Block, msg *types.SignedMessage,
		rcp *types.MessageReceipt) error {
		assert.Nil(rcp)
		return nil
	}

	testWaitHelp(nil, assert, node, sm1, false, msgApplySucc)
	testWaitHelp(nil, assert, node, sm2, false, msgApplyFail)
}

func TestGetSignature(t *testing.T) {
	require := require.New(t)
	t.Parallel()
	t.Run("no method", func(t *testing.T) {
		ctx := context.Background()
		assert := assert.New(t)

		nd := MakeNodesUnstarted(t, 1, true, true)[0]
		nodeAddr, err := nd.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
		)
		requireResetNodeGen(require, nd, tif)

		assert.NoError(nd.Start(ctx))
		defer nd.Stop(ctx)

		sig, err := nd.GetSignature(ctx, nodeAddr, "")
		assert.Equal(ErrNoMethod, err)
		assert.Nil(sig)
	})
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

func TestNextNonce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("account does not exist", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		node := MakeNodesUnstarted(t, 1, true, true)[0]
		nodeAddr, err := node.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
		)
		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		noActorAddress, err := node.NewAddress() // Won't have an actor.
		assert.NoError(err)

		_, err = NextNonce(ctx, node, noActorAddress)
		assert.Error(err)
		assert.Contains(err.Error(), "not found")
	})

	t.Run("account exists, largest value is in message pool", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		node := MakeNodesUnstarted(t, 1, true, true)[0]
		nodeAddr, err := node.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
		)
		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		// TODO: does sending a message to ourselves fit the spirit of the test?
		msg := types.NewMessage(nodeAddr, nodeAddr, 0, nil, "foo", []byte{})
		msg.Nonce = 42
		smsg, err := types.NewSignedMessage(*msg, node.Wallet)
		assert.NoError(err)
		core.MustAdd(node.MsgPool, smsg)

		nonce, err := NextNonce(ctx, node, nodeAddr)
		assert.NoError(err)
		assert.Equal(uint64(43), nonce)
	})
}

func TestNewMessageWithNextNonce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("includes correct nonce", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		node := MakeNodesUnstarted(t, 1, true, true)[0]
		nodeAddr, err := node.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
			consensus.ActorNonce(nodeAddr, 42),
		)

		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		bb := types.NewBlockForTest(node.ChainReader.Head().ToSlice()[0], 1)
		headTS := consensus.RequireNewTipSet(require, bb)
		chainForTest, ok := node.ChainReader.(chain.Store)
		require.True(ok)
		chain.RequirePutTsas(ctx, require, chainForTest, &chain.TipSetAndState{
			TipSet:          headTS,
			TipSetStateRoot: bb.StateRoot,
		})
		chainForTest.SetHead(ctx, headTS)

		msg, err := NewMessageWithNextNonce(ctx, node, nodeAddr, address.NewForTestGetter()(), nil, "foo", []byte{})
		require.NoError(err)
		assert.Equal(uint64(42), uint64(msg.Nonce))
	})
}

func TestQueryMessage(t *testing.T) {
	t.Parallel()

	t.Run("can contact payment broker", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		ctx := context.Background()

		node := MakeNodesUnstarted(t, 1, true, true)[0]
		nodeAddr, err := node.NewAddress()
		require.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
		)

		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		args, err := abi.ToEncodedValues(nodeAddr)
		require.NoError(err)

		returnValue, exitCode, err := node.CallQueryMethod(ctx, address.PaymentBrokerAddress, "ls", args, nil)
		require.NoError(err)
		require.Equal(uint8(0), exitCode)

		assert.NotNil(returnValue)
	})
}

func TestCreateMiner(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		node := MakeOfflineNode(t)
		nodeAddr, err := node.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(1000000)),
		)

		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		assert.Nil(node.SectorBuilder)

		result := <-RunCreateMiner(t, node, nodeAddr, uint64(100), th.RequireRandomPeerID(), *types.NewAttoFILFromFIL(100))
		require.NoError(result.Err)
		assert.NotNil(result.MinerAddress)

		assert.Equal(*result.MinerAddress, node.Repo.Config().Mining.MinerAddress)
	})

	t.Run("fail with pledge too low", func(t *testing.T) {
		node := MakeOfflineNode(t)
		nodeAddr, err := node.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
		)
		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		assert.Nil(node.SectorBuilder)

		result := <-RunCreateMiner(t, node, nodeAddr, uint64(1), th.RequireRandomPeerID(), *types.NewAttoFILFromFIL(10))
		assert.Error(result.Err)
		assert.Contains(result.Err.Error(), "pledge must be at least")
	})

	t.Run("fail with insufficient funds", func(t *testing.T) {
		node := MakeOfflineNode(t)
		nodeAddr, err := node.NewAddress()
		assert.NoError(err)

		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(nodeAddr, types.NewAttoFILFromFIL(10000)),
		)

		requireResetNodeGen(require, node, tif)

		assert.NoError(node.Start(ctx))

		assert.Nil(node.SectorBuilder)

		result := <-RunCreateMiner(t, node, nodeAddr, uint64(20), th.RequireRandomPeerID(), *types.NewAttoFILFromFIL(1000000))

		assert.Error(result.Err)
		assert.Contains(result.Err.Error(), "not enough balance")
	})
}

func TestLookupMinerAddress(t *testing.T) {
	t.Parallel()

	t.Run("lookup fails if provided address of non-miner actor", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)
		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, true, true)[0]
		addr := address.NewForTestGetter()()
		_, err := nd.Lookup.GetPeerIDByMinerAddress(ctx, addr)
		require.Error(err)
	})

	t.Run("lookup succeeds if provided address of a miner actor", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)
		ctx := context.Background()

		// Note: we should probably just have nodes make an address for themselves during init
		nd := MakeNodesUnstarted(t, 1, true, true)[0]
		minerOwnerAddr, err := nd.NewAddress()
		require.NoError(err)

		// initialize genesis block
		tif := consensus.MakeGenesisFunc(
			consensus.ActorAccount(minerOwnerAddr, types.NewAttoFILFromFIL(10000)),
		)

		requireResetNodeGen(require, nd, tif)

		newMinerPid := th.RequireRandomPeerID()

		require.NoError(nd.Start(ctx))

		// create a miner, owned by the account actor
		result := <-RunCreateMiner(t, nd, minerOwnerAddr, uint64(100), newMinerPid, *types.NewAttoFILFromFIL(100))
		require.NoError(result.Err)

		// retrieve the libp2p identity of the newly-created miner
		retPid, err := nd.Lookup.GetPeerIDByMinerAddress(ctx, *result.MinerAddress)
		require.NoError(err)
		require.Equal(peer.IDB58Encode(newMinerPid), peer.IDB58Encode(retPid))
	})
}

func TestDefaultMessageFromAddress(t *testing.T) {
	t.Run("it returns the configured wallet default if it exists", func(t *testing.T) {
		require := require.New(t)

		n := MakeOfflineNode(t)

		// generate a default address
		addrA, err := n.NewAddress()
		require.NoError(err)

		// load up the wallet with a few more addresses
		n.NewAddress()
		n.NewAddress()

		// configure a default
		n.Repo.Config().Wallet.DefaultAddress = addrA

		addrB, err := n.DefaultSenderAddress()
		require.NoError(err)
		require.Equal(addrA.String(), addrB.String())
	})

	/*
		t.Run("it returns an error if no default address was configured and more than one address in wallet", func(t *testing.T) {
			require := require.New(t)

			n := MakeOfflineNode(t)

			// generate a few addresses
			n.NewAddress()
			n.NewAddress()
			n.NewAddress()

			// remove existing wallet config
			n.Repo.Config().Wallet = &config.WalletConfig{}

			_, err := n.DefaultSenderAddress()
			require.Error(err)
			require.Equal(ErrNoDefaultMessageFromAddress, err)
		})
	*/
}
