package node

import (
	"context"
	"sync"
	"testing"
	"time"

	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
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

	nds := makeNodes(t, 2)
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

func TestNodeInit(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	r := repo.NewInMemoryRepo()
	assert.NoError(Init(ctx, r))

	nd, err := New(ctx, OptionsFromRepo(r)...)
	assert.NoError(err)

	assert.NoError(nd.Start())

	assert.NotNil(nd.ChainMgr.GetBestBlock())
	nd.Stop()
}

func TestNodeMining(t *testing.T) {
	assert := assert.New(t)
	newCid := types.NewCidForTestGetter()
	ctx := context.Background()
	node, err := New(ctx)
	assert.NoError(err)

	mockWorker := &mining.MockWorker{}
	inCh, outCh, doneWg := make(chan *types.Block), make(chan mining.Result), new(sync.WaitGroup)
	// Apparently you have to have exact types for testify.mock, so
	// we use iCh and oCh for the specific return type of Start().
	var iCh chan<- *types.Block = inCh
	var oCh <-chan mining.Result = outCh
	mockWorker.On("Start", mock.Anything).Return(iCh, oCh, doneWg)
	node.MiningWorker = mockWorker

	// Ensure that the initial input (the best block) is wired up properly.
	b1 := &types.Block{StateRoot: newCid()}
	var chainMgrForTest *core.ChainManagerForTest // nolint: gosimple, megacheck
	chainMgrForTest = node.ChainMgr
	chainMgrForTest.SetBestBlockForTest(ctx, b1)
	node.StartMining(ctx)
	gotBlock := <-inCh
	assert.True(b1.Cid().Equals(gotBlock.Cid()))

	// Ensure that the successive inputs (new best blocks) are wired up properly.
	b2 := &types.Block{StateRoot: newCid()}
	node.ChainMgr.SetBestBlockForTest(ctx, b2)
	gotBlock = <-inCh
	assert.True(b2.Cid().Equals(gotBlock.Cid()))

	// Ensure we're stopping cleanly.
	// Part of stopping cleanly is waiting for the worker to be done.
	// Kinda lame to test this way, but better than not testing.
	workerDone := false
	doneWg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		workerDone = true
		doneWg.Done()
	}()
	node.StopMining()
	assert.True(workerDone)
	// Part of stopping is ensuring we stop getting new blocks.
	assert.Equal(mining.ChannelClosed, mining.ReceiveInCh(inCh))
	node.ChainMgr.SetBestBlockForTest(ctx, b2)
	assert.Equal(mining.ChannelClosed, mining.ReceiveInCh(inCh))

	// Ensure that the output is wired up correctly.
	mockWorker = &mining.MockWorker{}
	inCh, outCh, doneWg = make(chan *types.Block), make(chan mining.Result), new(sync.WaitGroup)
	iCh = inCh
	oCh = outCh
	mockWorker.On("Start", mock.Anything).Return(iCh, oCh, doneWg)
	node.MiningWorker = mockWorker

	gotBlock = nil
	gotBlockCh := make(chan struct{})
	go func() { outCh <- mining.NewResult(b1, nil) }()
	node.startMining(ctx, func(ctx context.Context, b *types.Block) {
		gotBlock = b
		gotBlockCh <- struct{}{}
	})
	<-gotBlockCh
	node.StopMining()
	assert.True(b1.Cid().Equals(gotBlock.Cid()))
}
