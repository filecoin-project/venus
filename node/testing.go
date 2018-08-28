package node

import (
	"context"
	"fmt"
	"sync"
	"testing"

	hamt "gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	bserv "gx/ipfs/QmUSuYd5Q1N291DH679AVvHwGLwtS1V9VPDWvnUN9nGJPT/go-blockservice"
	offline "gx/ipfs/QmWdao8WJqYU65ZbYQyQWMFqku6QFxkPiv8HSUAkXdHZoe/go-ipfs-exchange-offline"
	pstore "gx/ipfs/QmZR2XWVVBCtbgBWnQhWk2xcQfaR3W8faQPriAiaaj7rsr/go-libp2p-peerstore"
	blockstore "gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/stretchr/testify/require"
)

type ChainSeed struct {
	info   *gengen.RenderedGenInfo
	cst    *hamt.CborIpldStore
	bstore blockstore.Blockstore
}

func MakeChainSeed(t *testing.T, cfg *gengen.GenesisCfg) *ChainSeed {
	t.Helper()

	// TODO: these six lines are ugly. We can do better...
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	cst := &hamt.CborIpldStore{Blocks: blkserv}

	info, err := gengen.GenGen(context.TODO(), cfg, cst, bstore)
	require.NoError(t, err)

	return &ChainSeed{
		info:   info,
		cst:    cst,
		bstore: bstore,
	}
}

func (cs *ChainSeed) GenesisInitFunc(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
	keys, err := cs.bstore.AllKeysChan(context.TODO())
	if err != nil {
		return nil, err
	}

	for k := range keys {
		blk, err := cs.bstore.Get(k)
		if err != nil {
			return nil, err
		}

		if err := bs.Put(blk); err != nil {
			return nil, err
		}
	}

	var blk types.Block
	if err := cst.Get(context.TODO(), cs.info.GenesisCid, &blk); err != nil {
		return nil, err
	}

	return &blk, nil
}

func (s *ChainSeed) GiveKey(t *testing.T, nd *Node, key string) types.Address {
	t.Helper()
	bcks := nd.Wallet.Backends(wallet.DSBackendType)
	require.Len(t, bcks, 1, "expected to get exactly one datastore backend")

	dsb := bcks[0].(*wallet.DSBackend)
	kinfo, ok := s.info.Keys[key]
	if !ok {
		t.Fatalf("Key %q does not exist in chain seed", key)
	}
	require.NoError(t, dsb.ImportKey(kinfo))

	addr, err := kinfo.Address()
	require.NoError(t, err)

	return addr
}

func (s *ChainSeed) GiveMiner(t *testing.T, nd *Node, which int) types.Address {
	t.Helper()
	cfg := nd.Repo.Config()
	m := s.info.Miners[which]

	cfg.Mining.MinerAddresses = append(cfg.Mining.MinerAddresses, m.Address)
	require.NoError(t, nd.Repo.ReplaceConfig(cfg))
	return m.Address
}

func (s *ChainSeed) Addr(t *testing.T, key string) types.Address {
	t.Helper()
	k, ok := s.info.Keys[key]
	if !ok {
		t.Fatal("no such key: ", key)
	}

	a, err := k.Address()
	if err != nil {
		t.Fatal(err)
	}

	return a
}

func NodesWithChainSeed(t *testing.T, n int, seed *ChainSeed) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		nd := genNode(t, false, true, seed.GenesisInitFunc, nil, nil)
		out = append(out, nd)
	}

	return out
}

func NodeWithChainSeed(t *testing.T, seed *ChainSeed, initopts ...InitOpt) *Node {
	t.Helper()
	return genNode(t, false, true, seed.GenesisInitFunc, initopts, nil)
}

func ConnectNodes(t *testing.T, a, b *Node) {
	t.Helper()
	pi := pstore.PeerInfo{
		ID:    b.Host.ID(),
		Addrs: b.Host.Addrs(),
	}

	err := a.Host.Connect(context.TODO(), pi)
	if err != nil {
		t.Fatal(err)
	}
}

// MakeNodesUnstarted creates n new (unstarted) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the initialized
// nodes
func MakeNodesUnstarted(t *testing.T, n int, offlineMode bool, mockMineMode bool, options ...func(c *Config) error) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		nd := genNode(t, offlineMode, mockMineMode, core.InitGenesis, nil, options)
		out = append(out, nd)
	}

	return out
}

func genNode(t *testing.T, offlineMode bool, mockMineMode bool, gif core.GenesisInitFunc, initopts []InitOpt, options []func(c *Config) error) *Node {
	r := repo.NewInMemoryRepo()
	r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

	err := Init(context.Background(), r, gif, initopts...)
	require.NoError(t, err)

	// set a random port here so things don't break in the event we make
	// a parallel request
	// TODO: can we use port 0 yet?
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

	// enables or disables libp2p
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
	return nd
}

// MakeNodesStarted creates n new (started) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the nodes
func MakeNodesStarted(t *testing.T, n int, offlineMode, mockMineMode bool) []*Node {
	t.Helper()
	nds := MakeNodesUnstarted(t, n, offlineMode, mockMineMode)
	for _, n := range nds {
		require.NoError(t, n.Start(context.Background()))
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
	getStateTree := func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
		return node.ChainMgr.State(ctx, ts.ToSlice())
	}
	w := mining.NewDefaultWorker(node.MsgPool, getStateTree, node.ChainMgr.Weight, core.ApplyMessages, node.ChainMgr.PwrTableView, node.Blockstore, node.CborStore, address.TestAddress, mining.BlockTimeTest)
	cur := node.ChainMgr.GetHeaviestTipSet()
	out := mining.MineOnce(ctx, mining.NewScheduler(w, mining.MineDelayTest), cur)
	require.NoError(out.Err)
	require.NoError(node.ChainMgr.SetHeaviestTipSetForTest(ctx, core.RequireNewTipSet(require, out.NewBlock)))

	require.NoError(err)

	return resultChan
}
