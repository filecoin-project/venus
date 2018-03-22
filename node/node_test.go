package node

import (
	"context"
	"sync"
	"testing"
	"time"

	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	types "github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNodeConstruct(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	nd, err := New(ctx)
	assert.NoError(err)
	assert.NotNil(nd.Host)

	nd.Stop()
}

func TestNodeNetworking(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	nd1, err := New(ctx, Libp2pOptions(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0")))
	assert.NoError(err)
	nd2, err := New(ctx, Libp2pOptions(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0")))
	assert.NoError(err)

	pinfo := peerstore.PeerInfo{
		ID:    nd2.Host.ID(),
		Addrs: nd2.Host.Addrs(),
	}

	err = nd1.Host.Connect(ctx, pinfo)
	assert.NoError(err)

	nd1.Stop()
	nd2.Stop()
}

func TestNodeMining(t *testing.T) {
	assert := assert.New(t)
	newCid := types.NewCidForTestGetter()
	ctx, _ := context.WithCancel(context.Background()) // nolint: vet
	node, err := New(ctx)
	assert.NoError(err)

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
	node.StartMining()
	gotInput := <-inCh
	assert.True(b1.Cid().Equals(gotInput.MineOn.Cid()))

	// Ensure that the successive inputs (new best blocks) are wired up properly.
	b2 := &types.Block{StateRoot: newCid()}
	node.ChainMgr.SetBestBlockForTest(ctx, b2)
	gotInput = <-inCh
	assert.True(b2.Cid().Equals(gotInput.MineOn.Cid()))

	// Ensure we don't mine when stopped.
	assert.Equal(mining.ChannelEmpty, mining.ReceiveInCh(inCh))
	node.StopMining()
	node.ChainMgr.SetBestBlockForTest(ctx, b2)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(mining.ChannelEmpty, mining.ReceiveInCh(inCh))

	// Ensure we're tearing down cleanly.
	// Part of stopping cleanly is waiting for the worker to be done.
	// Kinda lame to test this way, but better than not testing.
	node, err = New(ctx)
	assert.NoError(err)
	chainMgrForTest = node.ChainMgr
	chainMgrForTest.SetBestBlockForTest(ctx, b1)
	assert.NoError(node.Start())
	node.StartMining()
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
	ctx, _ = context.WithCancel(context.Background()) // nolint: vet
	node, err = New(ctx)
	assert.NoError(err)
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
	node.StartMining()
	go func() { outCh <- mining.NewOutput(b1, nil) }()
	<-gotBlockCh
	assert.True(b1.Cid().Equals(gotBlock.Cid()))
}

func TestUpdateMessagePool(t *testing.T) {
	// Note: majority of tests are in message_pool_test. This test
	// just makes sure it looks like it is hooked up correctly.
	assert := assert.New(t)
	ctx := context.Background()
	node, err := New(ctx)
	assert.NoError(err)
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
