package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	types "github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNodeConstruct(t *testing.T) {
	assert := assert.New(t)

	nd := MakeNodesUnstarted(t, 1, false)[0]
	assert.NotNil(nd.Host)

	nd.Stop()
}

func TestNodeNetworking(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	nds := MakeNodesUnstarted(t, 2, false)
	nd1, nd2 := nds[0], nds[1]

	pinfo := peerstore.PeerInfo{
		ID:    nd2.Host.ID(),
		Addrs: nd2.Host.Addrs(),
	}

	err := nd1.Host.Connect(ctx, pinfo)
	assert.NoError(err)

	nd1.Stop()
	nd2.Stop()
}

func TestConnectsToBootstrapNodes(t *testing.T) {
	ctx := context.Background()

	t.Run("no bootstrap nodes no problem", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		r := repo.NewInMemoryRepo()
		require.NoError(Init(ctx, r))
		r.Config().Bootstrap.Addresses = []string{}
		opts, err := OptionsFromRepo(r)
		require.NoError(err)

		nd, err := New(ctx, opts...)
		require.NoError(err)
		assert.NoError(nd.Start())
		defer nd.Stop()
	})

	t.Run("connects to bootstrap nodes", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		// These are two bootstrap nodes we'll connect to.
		nds := MakeNodesStarted(t, 2, false)
		nd1, nd2 := nds[0], nds[1]

		// Gotta be a better way to do this?
		peer1 := fmt.Sprintf("%s/ipfs/%s", nd1.Host.Addrs()[0].String(), nd1.Host.ID().Pretty())
		peer2 := fmt.Sprintf("%s/ipfs/%s", nd2.Host.Addrs()[0].String(), nd2.Host.ID().Pretty())

		// Create a node with the nodes above as bootstrap nodes.
		r := repo.NewInMemoryRepo()
		require.NoError(Init(ctx, r))
		r.Config().Bootstrap.Addresses = []string{peer1, peer2}
		opts, err := OptionsFromRepo(r)
		require.NoError(err)
		nd, err := New(ctx, opts...)
		require.NoError(err)
		nd.Bootstrapper.MinPeerThreshold = 2
		nd.Bootstrapper.Period = 10 * time.Millisecond
		assert.NoError(nd.Start())
		defer nd.Stop()

		// Ensure they're connected.
		time.Sleep(100 * time.Millisecond)
		assert.Len(nd.Host.Network().ConnsToPeer(nd1.Host.ID()), 1)
		assert.Len(nd.Host.Network().ConnsToPeer(nd2.Host.ID()), 1)
	})
}

func TestNodeInit(t *testing.T) {
	assert := assert.New(t)

	nd := MakeNodesUnstarted(t, 1, true)[0]

	assert.NoError(nd.Start())

	assert.NotNil(nd.ChainMgr.GetBestBlock())
	nd.Stop()
}

func TestStartMiningNoRewardAddress(t *testing.T) {
	assert := assert.New(t)

	nd := MakeNodesUnstarted(t, 1, true)[0]

	// remove default addr
	nd.rewardAddress = types.Address{}

	assert.NoError(nd.Start())
	err := nd.StartMining()
	assert.Error(err)
	assert.Contains(err.Error(), "no reward address")
}

func TestNodeMining(t *testing.T) {
	assert := assert.New(t)
	newCid := types.NewCidForTestGetter()
	ctx := context.Background()

	node := MakeNodesUnstarted(t, 1, true)[0]

	mockWorker := &mining.MockWorker{}
	inCh, outCh, doneWg := make(chan mining.Input), make(chan mining.Output), new(sync.WaitGroup)
	// Apparently you have to have exact types for testify.mock, so
	// we use iCh and oCh for the specific return type of Start().
	var iCh chan<- mining.Input = inCh
	var oCh <-chan mining.Output = outCh
	mockWorker.On("Start", mock.Anything).Return(iCh, oCh, doneWg)
	node.MiningWorker = mockWorker

	// Ensure that the initial input (the best block) is wired up properly.
	b1 := &types.Block{StateRoot: newCid()}
	var chainMgrForTest *core.ChainManagerForTest // nolint: gosimple, megacheck
	chainMgrForTest = node.ChainMgr
	chainMgrForTest.SetBestBlockForTest(ctx, b1)
	assert.NoError(node.Start())
	assert.NoError(node.StartMining())
	gotInput := <-inCh
	assert.True(b1.Cid().Equals(core.BaseBlockFromTipSets(gotInput.TipSets).Cid()))
	assert.Equal(node.Wallet.Addresses()[0].String(), gotInput.RewardAddress.String())

	// Ensure that the successive inputs (new best blocks) are wired up properly.
	b2 := &types.Block{StateRoot: newCid()}
	node.ChainMgr.SetBestBlockForTest(ctx, b2)
	gotInput = <-inCh
	assert.True(b2.Cid().Equals(core.BaseBlockFromTipSets(gotInput.TipSets).Cid()))

	// Ensure we don't mine when stopped.
	assert.Equal(mining.ChannelEmpty, mining.ReceiveInCh(inCh))
	node.StopMining()
	node.ChainMgr.SetBestBlockForTest(ctx, b2)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(mining.ChannelEmpty, mining.ReceiveInCh(inCh))

	// Ensure we're tearing down cleanly.
	// Part of stopping cleanly is waiting for the worker to be done.
	// Kinda lame to test this way, but better than not testing.
	node = MakeNodesUnstarted(t, 1, true)[0]

	chainMgrForTest = node.ChainMgr
	chainMgrForTest.SetBestBlockForTest(ctx, b1)
	assert.NoError(node.Start())
	assert.NoError(node.StartMining())
	workerDone := false
	node.miningDoneWg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		workerDone = true
		node.miningDoneWg.Done()
	}()
	node.Stop()
	assert.True(workerDone)

	// Ensure that the output is wired up correctly.
	node = MakeNodesUnstarted(t, 1, true)[0]

	mockWorker = &mining.MockWorker{}
	inCh, outCh, doneWg = make(chan mining.Input), make(chan mining.Output), new(sync.WaitGroup)
	iCh = inCh
	oCh = outCh
	mockWorker.On("Start", mock.Anything).Return(iCh, oCh, doneWg)
	node.MiningWorker = mockWorker
	assert.NoError(node.Start())

	var gotBlock *types.Block
	gotBlockCh := make(chan struct{})
	node.AddNewlyMinedBlock = func(ctx context.Context, b *types.Block) {
		gotBlock = b
		go func() { gotBlockCh <- struct{}{} }()
	}
	assert.NoError(node.StartMining())
	go func() { outCh <- mining.NewOutput(b1, nil) }()
	<-gotBlockCh
	assert.True(b1.Cid().Equals(gotBlock.Cid()))
}

func TestUpdateMessagePool(t *testing.T) {
	// Note: majority of tests are in message_pool_test. This test
	// just makes sure it looks like it is hooked up correctly.
	assert := assert.New(t)
	ctx := context.Background()
	node := MakeNodesUnstarted(t, 1, true)[0]

	var chainMgrForTest *core.ChainManagerForTest = node.ChainMgr // nolint: gosimple, megacheck, golint
	type msgs []*types.Message

	// Msg pool: [m0, m1],   Chain: b[m2, m3]
	// to
	// Msg pool: [m0, m3],   Chain: b[m1, m2]
	m := types.NewMsgs(4)
	core.MustAdd(node.MsgPool, m[0], m[1])
	oldChain := core.NewChainWithMessages(node.CborStore, nil, msgs{m[2], m[3]})
	newChain := core.NewChainWithMessages(node.CborStore, nil, msgs{m[1], m[2]})
	chainMgrForTest.SetBestBlockForTest(ctx, oldChain[len(oldChain)-1])
	assert.NoError(node.Start())
	updateMsgPoolDoneCh := make(chan struct{})
	node.BestBlockHandled = func() { updateMsgPoolDoneCh <- struct{}{} }
	// Triggers a notification, node should update the message pool as a result.
	chainMgrForTest.SetBestBlockForTest(ctx, newChain[len(newChain)-1])
	<-updateMsgPoolDoneCh
	assert.Equal(2, len(node.MsgPool.Pending()))
	pending := node.MsgPool.Pending()
	assert.True(types.MsgCidsEqual(m[0], pending[0]) || types.MsgCidsEqual(m[0], pending[1]))
	assert.True(types.MsgCidsEqual(m[3], pending[0]) || types.MsgCidsEqual(m[3], pending[1]))
	node.Stop()
}

func testWaitHelp(wg *sync.WaitGroup, assert *assert.Assertions, cm *core.ChainManager, expectMsg *types.Message,
	expectError bool) {
	expectCid, err := expectMsg.Cid()
	assert.NoError(err)

	err = cm.WaitForMessage(context.Background(), expectCid, func(b *types.Block, msg *types.Message,
		rcp *types.MessageReceipt) error {
		assert.True(types.MsgCidsEqual(expectMsg, msg))
		if wg != nil {
			wg.Done()
		}

		return nil
	})
	assert.Equal(expectError, err != nil)
}

type msgs []*types.Message

func TestWaitForMessage(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	node := MakeNodesUnstarted(t, 1, true)[0]

	err := node.Start()
	assert.NoError(err)

	stm := (*core.ChainManagerForTest)(node.ChainMgr)

	testWaitExisting(ctx, assert, node, stm)
	testWaitNew(ctx, assert, node, stm)
}

func TestWaitForMessageError(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	node := MakeNodesUnstarted(t, 1, true)[0]

	assert.NoError(node.Start())

	stm := (*core.ChainManagerForTest)(node.ChainMgr)

	testWaitError(ctx, assert, node, stm)
}

func testWaitExisting(ctx context.Context, assert *assert.Assertions, node *Node, stm *core.ChainManagerForTest) {
	newMsg := types.NewMessageForTestGetter()

	m1, m2 := newMsg(), newMsg()
	chain := core.NewChainWithMessages(node.CborStore, stm.GetBestBlock(), msgs{m1, m2})

	stm.SetBestBlockForTest(ctx, chain[len(chain)-1])

	testWaitHelp(nil, assert, stm, m1, false)
	testWaitHelp(nil, assert, stm, m2, false)
}

func testWaitNew(ctx context.Context, assert *assert.Assertions, node *Node,
	stm *core.ChainManagerForTest) {
	var wg sync.WaitGroup
	newMsg := types.NewMessageForTestGetter()

	m1, m2 := newMsg(), newMsg()
	chain := core.NewChainWithMessages(node.CborStore, stm.GetBestBlock(), msgs{m1, m2})

	wg.Add(2)
	go testWaitHelp(&wg, assert, stm, m1, false)
	go testWaitHelp(&wg, assert, stm, m2, false)
	time.Sleep(10 * time.Millisecond)

	stm.SetBestBlockForTest(ctx, chain[len(chain)-1])
	wg.Wait()
}

func testWaitError(ctx context.Context, assert *assert.Assertions, node *Node, stm *core.ChainManagerForTest) {
	newMsg := types.NewMessageForTestGetter()

	stm.FetchBlock = func(ctx context.Context, cid *cid.Cid) (*types.Block, error) {
		return nil, fmt.Errorf("error fetching block (in test)")
	}

	m1, m2, m3, m4 := newMsg(), newMsg(), newMsg(), newMsg()
	chain := core.NewChainWithMessages(node.CborStore, stm.GetBestBlock(), msgs{m1, m2})
	chain2 := core.NewChainWithMessages(node.CborStore, chain[len(chain)-1], msgs{m3, m4})
	stm.SetBestBlockForTest(ctx, chain2[len(chain2)-1])

	testWaitHelp(nil, assert, stm, m2, true)
}

func TestGetSignature(t *testing.T) {
	t.Run("no method", func(t *testing.T) {
		ctx := context.Background()
		assert := assert.New(t)

		nd := MakeNodesUnstarted(t, 1, true)[0]
		assert.NoError(nd.Start())
		defer nd.Stop()

		sig, err := nd.GetSignature(ctx, address.TestAddress, "")
		assert.Equal(ErrNoMethod, err)
		assert.Nil(sig)
	})
}

func TestOptionWithError(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)
	r := repo.NewInMemoryRepo()
	assert.NoError(Init(ctx, r))

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
	ctx := context.Background()

	t.Run("account does not exist", func(t *testing.T) {
		assert := assert.New(t)
		node := MakeNodesUnstarted(t, 1, true)[0]
		err := node.ChainMgr.Genesis(ctx, core.InitGenesis)
		assert.NoError(err)
		assert.NoError(node.Start())

		address := types.NewAddressForTestGetter()() // Won't have an actor.

		_, err = NextNonce(ctx, node, address)
		assert.Error(err)
		assert.Contains(err.Error(), "not found")
	})

	t.Run("account exists, largest value is in message pool", func(t *testing.T) {
		assert := assert.New(t)
		node := MakeNodesUnstarted(t, 1, true)[0]
		err := node.ChainMgr.Genesis(ctx, core.InitGenesis)
		assert.NoError(err)
		assert.NoError(node.Start())

		addr := address.TestAddress // Has an actor.
		msg := types.NewMessage(addr, address.TestAddress, 0, nil, "foo", []byte{})
		msg.Nonce = 42
		core.MustAdd(node.MsgPool, msg)

		nonce, err := NextNonce(ctx, node, addr)
		assert.NoError(err)
		assert.Equal(uint64(43), nonce)
	})
}

func TestNewMessageWithNextNonce(t *testing.T) {
	ctx := context.Background()

	t.Run("includes correct nonce", func(t *testing.T) {
		assert := assert.New(t)
		node := MakeNodesUnstarted(t, 1, true)[0]
		err := node.ChainMgr.Genesis(ctx, core.InitGenesis)
		assert.NoError(err)
		assert.NoError(node.Start())

		address := address.TestAddress // Has an actor.

		bb := types.NewBlockForTest(node.ChainMgr.GetBestBlock(), 1)
		st, err := state.LoadStateTree(context.Background(), node.CborStore, bb.StateRoot, nil)
		assert.NoError(err)
		actor := state.MustGetActor(st, address)
		actor.Nonce = 42
		cid := state.MustSetActor(st, address, actor)
		bb.StateRoot = cid
		var chainMgrForTest *core.ChainManagerForTest = node.ChainMgr // nolint: golint
		chainMgrForTest.SetBestBlockForTest(ctx, bb)

		msg, err := NewMessageWithNextNonce(ctx, node, address, types.NewAddressForTestGetter()(), nil, "foo", []byte{})
		assert.NoError(err)
		assert.Equal(uint64(42), msg.Nonce)
	})
}

func TestQueryMessage(t *testing.T) {
	ctx := context.Background()

	t.Run("can contact payment broker", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)
		node := MakeNodesUnstarted(t, 1, true)[0]
		err := node.ChainMgr.Genesis(ctx, core.InitGenesis)
		assert.NoError(err)
		assert.NoError(node.Start())

		params, err := abi.ToEncodedValues(address.TestAddress)
		require.NoError(err)
		msg, err := NewMessageWithNextNonce(ctx, node, address.TestAddress, address.PaymentBrokerAddress, nil, "ls", params)
		require.NoError(err)

		returnValue, exitCode, err := node.QueryMessage(msg)
		require.NoError(err)
		require.Equal(uint8(0), exitCode)

		assert.NotNil(returnValue)
	})
}
