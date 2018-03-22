package node

import (
	"context"
	"testing"
	"time"

	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	peerstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func makeNodes(t *testing.T, n int) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		r := repo.NewInMemoryRepo()
		if err := Init(context.Background(), r); err != nil {
			t.Fatal(err)
		}
		opts := append(OptionsFromRepo(r), Libp2pOptions(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0")))
		nd, err := New(context.Background(), opts...)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, nd)
	}

	return out
}

func connect(t *testing.T, nd1, nd2 *Node) {
	t.Helper()
	pinfo := peerstore.PeerInfo{
		ID:    nd2.Host.ID(),
		Addrs: nd2.Host.Addrs(),
	}

	if err := nd1.Host.Connect(context.Background(), pinfo); err != nil {
		t.Fatal(err)
	}
}

func stopNodes(nds []*Node) {
	for _, nd := range nds {
		nd.Stop()
	}
}

func startNodes(t *testing.T, nds []*Node) {
	t.Helper()
	for _, nd := range nds {
		if err := nd.Start(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBlockPropTwoNodes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	assert := assert.New(t)

	nodes := makeNodes(t, 2)
	startNodes(t, nodes)
	defer stopNodes(nodes)
	connect(t, nodes[0], nodes[1])

	baseBlk := nodes[0].ChainMgr.GetBestBlock()
	nextBlk := &types.Block{Parent: baseBlk.Cid(), Height: 1, StateRoot: baseBlk.StateRoot}

	// Wait for network connection notifications to propagate
	time.Sleep(time.Millisecond * 50)

	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk))

	time.Sleep(time.Millisecond * 50)

	otherBest := nodes[1].ChainMgr.GetBestBlock()
	assert.Equal(otherBest.Cid(), nextBlk.Cid())
}

func TestChainSync(t *testing.T) {
	ctx := context.Background()
	assert := assert.New(t)

	nodes := makeNodes(t, 2)
	startNodes(t, nodes)
	defer stopNodes(nodes)

	baseBlk := nodes[0].ChainMgr.GetBestBlock()
	nextBlk1 := &types.Block{Parent: baseBlk.Cid(), Height: 1, StateRoot: baseBlk.StateRoot}
	nextBlk2 := &types.Block{Parent: nextBlk1.Cid(), Height: 2, StateRoot: baseBlk.StateRoot}
	nextBlk3 := &types.Block{Parent: nextBlk2.Cid(), Height: 3, StateRoot: baseBlk.StateRoot}

	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk1))
	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk2))
	assert.NoError(nodes[0].AddNewBlock(ctx, nextBlk3))

	connect(t, nodes[0], nodes[1])

	time.Sleep(time.Millisecond * 50)
	otherBest := nodes[1].ChainMgr.GetBestBlock()
	assert.Equal(otherBest.Cid(), nextBlk3.Cid())
}
