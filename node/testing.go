package node

import (
	"context"
	"sync"
	"testing"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/require"
)

// MakeNodesUnstarted creates n new (unstarted) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the initialized
// nodes
func MakeNodesUnstarted(t *testing.T, n int, offlineMode bool) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		r := repo.NewInMemoryRepo()
		err := Init(context.Background(), r)
		require.NoError(t, err)

		if !offlineMode {
			r.Config().Swarm.Address = "/ip4/127.0.0.1/tcp/0"
		}

		opts, err := OptionsFromRepo(r)
		require.NoError(t, err)

		// disables libp2p
		opts = append(opts, func(c *Config) error {
			c.OfflineMode = offlineMode
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
func MakeNodesStarted(t *testing.T, n int, offlineMode bool) []*Node {
	t.Helper()
	nds := MakeNodesUnstarted(t, n, offlineMode)
	for _, n := range nds {
		require.NoError(t, n.Start())
	}
	return nds
}

// MakeOfflineNode returns a single unstarted offline node.
func MakeOfflineNode(t *testing.T) *Node {
	return MakeNodesUnstarted(t, 1, true)[0]
}

// MustCreateMiner creates a miner owned by address.TestAddress and returns its address. It requires that the node has
// been initialized with a genesis block and that it has been started.
func MustCreateMiner(t *testing.T, node *Node) types.Address {
	require := require.New(t)
	result := <-RunCreateMiner(t, node, address.TestAddress, *types.NewBytesAmount(100000), *types.NewTokenAmount(100))

	require.NoError(result.err)

	return *result.minerAddress
}

// MustCreateMinerResult contains the result of a CreateMiner command
type MustCreateMinerResult struct {
	minerAddress *types.Address
	err          error
}

// RunCreateMiner runs create miner and then runs a given assertion with the result.
func RunCreateMiner(t *testing.T, node *Node, from types.Address, pledge types.BytesAmount, collateral types.TokenAmount) chan MustCreateMinerResult {
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
		minerAddr, err := node.CreateMiner(ctx, from, pledge, collateral)
		resultChan <- MustCreateMinerResult{minerAddress: minerAddr, err: err}
		wg.Done()
	}()

	// wait for create miner call to put a message in the pool
	_, err = subscription.Next(ctx)
	require.NoError(err)

	blockGenerator := mining.NewBlockGenerator(node.MsgPool, func(ctx context.Context, cid *cid.Cid) (state.Tree, error) {
		return state.LoadStateTree(ctx, node.CborStore, cid, builtin.Actors)
	}, mining.ApplyMessages)
	cur := node.ChainMgr.GetBestBlock()
	out := mining.MineOnce(ctx, mining.NewWorker(blockGenerator), []core.TipSet{{cur.Cid().String(): cur}}, address.TestAddress)
	require.NoError(out.Err)
	require.NoError(node.ChainMgr.SetBestBlockForTest(ctx, out.NewBlock))

	require.NoError(err)

	return resultChan
}
