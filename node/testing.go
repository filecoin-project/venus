package node

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/require"
)

// MakeNodesUnstarted creates n new (unstarted) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the initialized
// nodes
func MakeNodesUnstarted(t *testing.T, n int, offlineMode bool, mockMineMode bool, options ...func(c *Config) error) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		r := repo.NewInMemoryRepo()
		err := Init(context.Background(), r, core.InitGenesis)
		require.NoError(t, err)

		// set a random port here so things don't break in the event we make
		// a parallel request
		port, err := th.GetFreePort()
		require.NoError(t, err)
		r.Config().API.Address = fmt.Sprintf(":%d", port)

		if !offlineMode {
			r.Config().Swarm.Address = "/ip4/127.0.0.1/tcp/0"
		}

		opts, err := OptionsFromRepo(r)
		require.NoError(t, err)

		for _, o := range options {
			opts = append(opts, o)
		}

		// disables libp2p
		opts = append(opts, func(c *Config) error {
			c.OfflineMode = offlineMode
			return nil
		})

		opts = append(opts, func(c *Config) error {
			c.MockMineMode = mockMineMode
			return nil
		})

		nd, err := New(context.Background(), opts...)
		require.NoError(t, err)
		out = append(out, nd)
	}

	return out
}

// MakeNodesStarted creates n new (started) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the nodes
func MakeNodesStarted(t *testing.T, n int, offlineMode, mockMineMode bool) []*Node {
	t.Helper()
	nds := MakeNodesUnstarted(t, n, offlineMode, mockMineMode)
	for _, n := range nds {
		require.NoError(t, n.Start())
	}
	return nds
}

// MakeOfflineNode returns a single unstarted offline node with mocked mining.
func MakeOfflineNode(t *testing.T) *Node {
	return MakeNodesUnstarted(t, 1, true, true)[0]
}

// MustCreateMinerResult contains the result of a CreateMiner command
type MustCreateMinerResult struct {
	MinerAddress *types.Address
	Err          error
}

// RunCreateMiner runs create miner and then runs a given assertion with the result.
func RunCreateMiner(t *testing.T, node *Node, from types.Address, pledge types.BytesAmount, pid peer.ID, collateral types.AttoFIL) chan MustCreateMinerResult {
	resultChan := make(chan MustCreateMinerResult)
	require := require.New(t)

	if node.ChainMgr.GetGenesisCid() == nil {
		panic("must initialize with genesis block first")
	}

	ctx := context.Background()

	var wg sync.WaitGroup

	wg.Add(1)

	subscription, err := node.PubSub.Subscribe(MessageTopic)
	require.NoError(err)

	go func() {
		minerAddr, err := node.CreateMiner(ctx, from, pledge, pid, collateral)
		resultChan <- MustCreateMinerResult{MinerAddress: minerAddr, Err: err}
		wg.Done()
	}()

	// wait for create miner call to put a message in the pool
	_, err = subscription.Next(ctx)
	require.NoError(err)

	blockGenerator := mining.NewBlockGenerator(node.MsgPool, func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
		return node.ChainMgr.State(ctx, ts.ToSlice())
	}, node.ChainMgr.Weight, core.ApplyMessages)
	cur := node.ChainMgr.GetHeaviestTipSet()
	out := mining.MineOnce(ctx, mining.NewWorker(blockGenerator), cur, address.TestAddress)
	require.NoError(out.Err)
	require.NoError(node.ChainMgr.SetHeaviestTipSetForTest(ctx, core.RequireNewTipSet(require, out.NewBlock)))

	require.NoError(err)

	return resultChan
}
