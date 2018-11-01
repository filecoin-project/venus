package node

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	hamt "gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	offline "gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	blockstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	pstore "gx/ipfs/QmeKD8YT7887Xu6Z86iZmpYNxrLogJexqxEugSmaf14k64/go-libp2p-peerstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/message"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/protocol/storage"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ChainSeed is
type ChainSeed struct {
	info   *gengen.RenderedGenInfo
	cst    *hamt.CborIpldStore
	bstore blockstore.Blockstore
}

// MakeChainSeed is
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

// GenesisInitFunc is a th.GenesisInitFunc using the chain seed
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

// GiveKey gives the given key to the given node
func (cs *ChainSeed) GiveKey(t *testing.T, nd *Node, key int) address.Address {
	t.Helper()
	bcks := nd.Wallet.Backends(wallet.DSBackendType)
	require.Len(t, bcks, 1, "expected to get exactly one datastore backend")

	dsb := bcks[0].(*wallet.DSBackend)
	kinfo := cs.info.Keys[key]
	require.NoError(t, dsb.ImportKey(kinfo))

	addr, err := kinfo.Address()
	require.NoError(t, err)

	return addr
}

// GiveMiner gives the specified miner to the node. Returns the address and the owner addresss
func (cs *ChainSeed) GiveMiner(t *testing.T, nd *Node, which int) (address.Address, address.Address) {
	t.Helper()
	cfg := nd.Repo.Config()
	m := cs.info.Miners[which]

	cfg.Mining.MinerAddress = m.Address
	require.NoError(t, nd.Repo.ReplaceConfig(cfg))

	ownerAddr, err := cs.info.Keys[m.Owner].Address()
	require.NoError(t, err)

	return m.Address, ownerAddr
}

// Addr returns the address for the given key
func (cs *ChainSeed) Addr(t *testing.T, key int) address.Address {
	t.Helper()
	k := cs.info.Keys[key]

	a, err := k.Address()
	if err != nil {
		t.Fatal(err)
	}

	return a
}

// NodesWithChainSeed creates some nodes using the given chain seed
func NodesWithChainSeed(t *testing.T, n int, seed *ChainSeed) []*Node {
	t.Helper()
	var out []*Node
	for i := 0; i < n; i++ {
		nd := genNode(t, false, true, seed.GenesisInitFunc, nil, nil)
		out = append(out, nd)
	}

	return out
}

// NodeWithChainSeed makes a single node with the given chain seed, and some init options
func NodeWithChainSeed(t *testing.T, seed *ChainSeed, initopts ...InitOpt) *Node { // nolint: golint
	t.Helper()
	return genNode(t, false, false, seed.GenesisInitFunc, initopts, nil)
}

// ConnectNodes connects two nodes together
func ConnectNodes(t *testing.T, a, b *Node) {
	t.Helper()
	pi := pstore.PeerInfo{
		ID:    b.Host().ID(),
		Addrs: b.Host().Addrs(),
	}

	err := a.Host().Connect(context.TODO(), pi)
	if err != nil {
		t.Fatal(err)
	}
}

// MakeNodesUnstartedWithGif creates a new (unstarted) nodes with an
// InMemoryRepo initialized with the given genesis init function, applies
// options from the InMemory Repo and returns a slice of the initialized nodes.
func MakeNodesUnstartedWithGif(t *testing.T, n int, offlineMode bool, mockMineMode bool, gif consensus.GenesisInitFunc, options ...func(c *Config) error) []*Node {
	var out []*Node
	for i := 0; i < n; i++ {
		nd := genNode(t, offlineMode, mockMineMode, gif, nil, options)
		out = append(out, nd)
	}

	return out
}

// MakeNodeUnstartedSeed creates a new node with seeded setup.
func MakeNodeUnstartedSeed(t *testing.T, offlineMode bool, mockMineMode bool, options ...func(c *Config) error) *Node {
	seed := MakeChainSeed(t, TestGenCfg)
	node := genNode(t, offlineMode, mockMineMode, seed.GenesisInitFunc, nil, options)
	seed.GiveKey(t, node, 0)
	minerAddr, minerOwnerAddr := seed.GiveMiner(t, node, 0)
	_, err := storage.NewMiner(context.Background(), minerAddr, minerOwnerAddr, node)
	assert.NoError(t, err)

	return node
}

// ensurePerformRealProofsDefaultsToTrue appends an InitOpt which causes the
// node to perform real proofs and use small sector sizes if no InitOpt of
// that type already exists in the provided `initopts` slice.
func ensurePerformRealProofsDefaultsToTrue(initopts []InitOpt) []InitOpt {
	cfg := new(InitCfg)
	cfg.PerformRealProofs = true

	for _, o := range initopts {
		o(cfg)
	}

	if !cfg.PerformRealProofs {
		// an InitOpt was provided which disabled real
		// proofs; eject
		return initopts
	}

	cfg.PerformRealProofs = false

	for _, o := range initopts {
		o(cfg)
	}

	if cfg.PerformRealProofs {
		// an InitOpt was provided which enabled real
		// proofs; eject
		return initopts
	}

	// no InitOpt for configuring proofs was provided,
	// so we add one here
	return append(initopts, PerformRealProofsOpt(true))
}

func genNode(t *testing.T, offlineMode bool, mockMineMode bool, gif consensus.GenesisInitFunc, initopts []InitOpt, options []func(c *Config) error) *Node {
	r := repo.NewInMemoryRepo()
	r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"

	err := Init(context.Background(), r, gif, ensurePerformRealProofsDefaultsToTrue(initopts)...)
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

	nd, err := New(context.Background(), opts...)

	if mockMineMode {
		nd.PowerTable = &consensus.TestView{}
		newCon := consensus.NewExpected(nd.CborStore(), nd.Blockstore, nd.PowerTable, nd.ChainReader.GenesisCid())
		newChainStore, ok := nd.ChainReader.(chain.Store)
		require.True(t, ok)

		newSyncer := chain.NewDefaultSyncer(nd.OnlineStore, nd.CborStore(), newCon, newChainStore)
		nd.Syncer = newSyncer
		nd.Consensus = newCon
	}

	require.NoError(t, err)
	return nd
}

// MakeNodesUnstarted creates n new (unstarted) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the initialized
// nodes
func MakeNodesUnstarted(t *testing.T, n int, offlineMode bool, mockMineMode bool, options ...func(c *Config) error) []*Node {
	return MakeNodesUnstartedWithGif(t, n, offlineMode, mockMineMode, consensus.InitGenesis, options...)
}

// MakeNodesStartedWithGif creates n new (started) nodes with an
// InMemoryRepo initialized with the given genesis init function, applies
// options from the InMemory Repo and returns a slice of the nodes.
func MakeNodesStartedWithGif(t *testing.T, n int, offlineMode bool, mockMineMode bool, gif consensus.GenesisInitFunc) []*Node {
	t.Helper()
	nds := MakeNodesUnstartedWithGif(t, n, offlineMode, mockMineMode, gif)
	for _, n := range nds {
		require.NoError(t, n.Start(context.Background()))
	}
	return nds
}

// MakeNodesStarted creates n new (started) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the nodes
func MakeNodesStarted(t *testing.T, n int, offlineMode, mockMineMode bool) []*Node {
	return MakeNodesStartedWithGif(t, n, offlineMode, mockMineMode, consensus.InitGenesis)
}

// MakeOfflineNode returns a single unstarted offline node with mocked mining.
func MakeOfflineNode(t *testing.T) *Node {
	return MakeNodesUnstarted(t, 1, true, true)[0]
}

// MustCreateMinerResult contains the result of a CreateMiner command
type MustCreateMinerResult struct {
	MinerAddress *address.Address
	Err          error
}

// RunCreateMiner runs create miner and then runs a given assertion with the result.
func RunCreateMiner(t *testing.T, node *Node, from address.Address, pledge uint64, pid peer.ID, collateral types.AttoFIL) chan MustCreateMinerResult {
	resultChan := make(chan MustCreateMinerResult)
	require := require.New(t)

	if node.ChainReader.GenesisCid() == nil {
		panic("must initialize with genesis block first")
	}

	ctx := context.Background()

	var wg sync.WaitGroup

	wg.Add(1)

	subscription, err := node.PubSub.Subscribe(MessageTopic)
	require.NoError(err)

	go func() {
		minerAddr, err := node.CreateMiner(ctx, from, pledge, pid, &collateral)
		resultChan <- MustCreateMinerResult{MinerAddress: minerAddr, Err: err}
		wg.Done()
	}()

	// wait for create miner call to put a message in the pool
	_, err = subscription.Next(ctx)
	require.NoError(err)
	getStateFromKey := func(ctx context.Context, tsKey string) (state.Tree, error) {
		tsas, err := node.ChainReader.GetTipSetAndState(ctx, tsKey)
		if err != nil {
			return nil, err
		}
		return state.LoadStateTree(ctx, node.CborStore(), tsas.TipSetStateRoot, builtin.Actors)
	}
	getStateTree := func(ctx context.Context, ts consensus.TipSet) (state.Tree, error) {
		return getStateFromKey(ctx, ts.String())
	}
	getWeight := func(ctx context.Context, ts consensus.TipSet) (uint64, uint64, error) {
		parent, err := ts.Parents()
		if err != nil {
			return uint64(0), uint64(0), err
		}
		// TODO handle genesis cid more gracefully
		if parent.Len() == 0 {
			return node.Consensus.Weight(ctx, ts, nil)
		}
		pSt, err := getStateFromKey(ctx, parent.String())
		if err != nil {
			return uint64(0), uint64(0), err
		}
		return node.Consensus.Weight(ctx, ts, pSt)
	}

	w := mining.NewDefaultWorker(node.MsgPool, getStateTree, getWeight, consensus.ApplyMessages, node.PowerTable, node.Blockstore, node.CborStore(), address.TestAddress, th.BlockTimeTest)
	cur := node.ChainReader.Head()
	out := mining.MineOnce(ctx, w, mining.MineDelayTest, cur)
	require.NoError(out.Err)
	outTS := consensus.RequireNewTipSet(require, out.NewBlock)
	chainStore, ok := node.ChainReader.(chain.Store)
	require.True(ok)
	tsas := &chain.TipSetAndState{
		TipSet:          outTS,
		TipSetStateRoot: out.NewBlock.StateRoot,
	}
	require.NoError(chainStore.PutTipSetAndState(ctx, tsas))
	require.NoError(chainStore.SetHead(ctx, outTS))
	return resultChan
}

func requireResetNodeGen(require *require.Assertions, node *Node, gif consensus.GenesisInitFunc) { // nolint: deadcode
	require.NoError(resetNodeGen(node, gif))
}

// resetNodeGen resets the genesis block of the input given node using the gif
// function provided.
func resetNodeGen(node *Node, gif consensus.GenesisInitFunc) error {
	ctx := context.Background()
	newGenBlk, err := gif(node.CborStore(), node.Blockstore)
	if err != nil {
		return err
	}
	newGenTS, err := consensus.NewTipSet(newGenBlk)
	if err != nil {
		return errors.Wrap(err, "failed to generate genesis block")
	}
	// Persist the genesis tipset to the repo.
	genTsas := &chain.TipSetAndState{
		TipSet:          newGenTS,
		TipSetStateRoot: newGenBlk.StateRoot,
	}

	var newChainStore chain.Store = chain.NewDefaultStore(node.Repo.ChainDatastore(), node.CborStore(), newGenBlk.Cid())

	if err = newChainStore.PutTipSetAndState(ctx, genTsas); err != nil {
		return errors.Wrap(err, "failed to put genesis block in chain store")
	}
	if err = newChainStore.SetHead(ctx, newGenTS); err != nil {
		return errors.Wrap(err, "failed to persist genesis block in chain store")
	}
	// Persist the genesis cid to the repo.
	val, err := json.Marshal(newGenBlk.Cid())
	if err != nil {
		return errors.Wrap(err, "failed to marshal genesis cid")
	}
	if err = node.Repo.Datastore().Put(chain.GenesisKey, val); err != nil {
		return errors.Wrap(err, "failed to persist genesis cid")
	}
	newChainReader, ok := newChainStore.(chain.ReadStore)
	if !ok {
		return errors.New("failed to cast chain.Store to chain.ReadStore")
	}
	newCon := consensus.NewExpected(node.CborStore(), node.Blockstore, node.PowerTable, newGenBlk.Cid())
	newSyncer := chain.NewDefaultSyncer(node.OnlineStore, node.CborStore(), newCon, newChainStore)
	newMsgWaiter := message.NewWaiter(newChainReader, node.Blockstore, node.CborStore())
	node.ChainReader = newChainReader
	node.Consensus = newCon
	node.Syncer = newSyncer
	node.MessageWaiter = newMsgWaiter
	node.lookup = lookup.NewChainLookupService(newChainReader, node.DefaultSenderAddress, node.Blockstore)
	return nil
}

// PeerKeys are a list of keys for peers that can be used in testing.
var PeerKeys = []crypto.PrivKey{
	mustGenKey(101),
}

// TestGenCfg is a genesis configuration used for tests.
var TestGenCfg = &gengen.GenesisCfg{
	Keys: 2,
	Miners: []gengen.Miner{
		{
			Owner:  0,
			Power:  100,
			PeerID: mustPeerID(PeerKeys[0]).Pretty(),
		},
	},
	PreAlloc: []string{
		"10000",
		"10000",
	},
}

func mustGenKey(seed int64) crypto.PrivKey {
	r := rand.New(rand.NewSource(seed))
	priv, _, err := crypto.GenerateEd25519Key(r)
	if err != nil {
		panic(err)
	}

	return priv
}

func mustPeerID(k crypto.PrivKey) peer.ID {
	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		panic(err)
	}
	return pid
}
