package node

import (
	"context"
	"fmt"
	"encoding/json"
	"math/rand"
	"os"
	"sync"
	"testing"

	"gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	pstore "gx/ipfs/QmQAGG1zxfePqj2t7bLxyN8AFccZ889DDR9Gn8kVLDrGZo/go-libp2p-peerstore"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	bserv "gx/ipfs/QmVDTbzzTwnuBwNbJdhW3u7LoBQp46bezm9yp4z1RoEepM/go-blockservice"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmYZwey1thDTynSrvd6qQkX24UpTka6TFhQ2v569UpoqxD/go-ipfs-exchange-offline"
	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	api2impl "github.com/filecoin-project/go-filecoin/api2/impl"
	"github.com/filecoin-project/go-filecoin/api2/impl/mthdsigapi"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/lookup"
	"github.com/filecoin-project/go-filecoin/message"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"

	"github.com/stretchr/testify/require"
)

// ChainSeed is a generalized struct for configuring node
type ChainSeed struct {
	info   *gengen.RenderedGenInfo
	cst    *hamt.CborIpldStore
	bstore blockstore.Blockstore
}

// TestNodeOptions is a generalized struct for passing Node options around for testing
type TestNodeOptions struct {
	OfflineMode bool
	ConfigOpts  []ConfigOpt
	GenesisFunc consensus.GenesisInitFunc
	InitOpts    []InitOpt
	Seed        *ChainSeed
}

// MakeChainSeed creates a chain seed struct (see above) from a given
// genesis config
func MakeChainSeed(t *testing.T, cfg *gengen.GenesisCfg) *ChainSeed {
	t.Helper()

	// TODO: these six lines are ugly. We can do better...
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bstore)
	blkserv := bserv.New(bstore, offl)
	cst := &hamt.CborIpldStore{Blocks: blkserv}

	info, err := gengen.GenGen(context.TODO(), cfg, cst, bstore, 0)
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

// MakeNodeWithChainSeed makes a single node with the given chain seed, and some init options
func MakeNodeWithChainSeed(t *testing.T, seed *ChainSeed, initopts ...InitOpt) *Node { // nolint: golint
	t.Helper()
	tno := TestNodeOptions{OfflineMode: false, GenesisFunc: seed.GenesisInitFunc, InitOpts: initopts}
	return GenNode(t, &tno)
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
func MakeNodesUnstartedWithGif(t *testing.T, numNodes int, offlineMode bool, gif consensus.GenesisInitFunc, options []ConfigOpt) []*Node {
	var out []*Node

	tno := TestNodeOptions{
		OfflineMode: offlineMode,
		GenesisFunc: gif,
		ConfigOpts:  options,
	}

	for i := 0; i < numNodes; i++ {
		nd := GenNode(t, &tno)
		out = append(out, nd)
	}

	return out
}

// MakeNodesUnstarted creates n new (unstarted) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the initialized
// nodes
func MakeNodesUnstarted(t *testing.T, numNodes int, offlineMode bool, mockMineMode bool) []*Node {
	var configOpts []ConfigOpt

	if mockMineMode {
		configOpts = configureFakeProver(configOpts)
	}

	return MakeNodesUnstartedWithGif(t, numNodes, offlineMode, consensus.InitGenesis, configOpts)
}

// MakeNodesStarted creates n new (started) nodes with an InMemoryRepo,
// applies options from the InMemoryRepo and returns a slice of the nodes
func MakeNodesStarted(t *testing.T, numNodes int, offlineMode, mockMineMode bool) []*Node {
	var nds []*Node

	var configOpts []ConfigOpt
	if mockMineMode {
		configOpts = configureFakeProver(configOpts)
	}

	nds = MakeNodesUnstartedWithGif(t, numNodes, offlineMode, consensus.InitGenesis, configOpts)
	for _, n := range nds {
		require.NoError(t, n.Start(context.Background()))
	}
	return nds
}

// MakeOfflineNode returns a single unstarted offline node with mocked mining.
func MakeOfflineNode(t *testing.T) *Node {
	return MakeNodesUnstartedWithGif(t,
		1,    /* 1 node */
		true, /* offline */
		consensus.InitGenesis,
		nil /* default config */)[0]
}

// MustCreateMinerResult contains the result of a CreateMiner command
type MustCreateMinerResult struct {
	MinerAddress *address.Address
	Err          error
}

func configureFakeProver(cfo []ConfigOpt) []ConfigOpt {
	prover := proofs.NewFakeProver(true, nil)
	return append(cfo, ProverConfigOption(prover))
}

// resetNodeGen resets the genesis block of the input given node using the gif
// function provided.
// Note: this is an awful way to test the node. This function duplicates to a large
// degree what the constructor does. It should not be in the business of replacing
// fields on node as doing that correctly requires knowing exactly how the node is
// created, which is bad information to need to rely on in tests.
func resetNodeGen(node *Node, gif consensus.GenesisInitFunc) error { // nolint: deadcode
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
	newCon := consensus.NewExpected(node.CborStore(),
		node.Blockstore,
		node.PowerTable,
		newGenBlk.Cid(),
		proofs.NewFakeProver(true, nil))
	newSyncer := chain.NewDefaultSyncer(node.OnlineStore, node.CborStore(), newCon, newChainStore)
	node.ChainReader = newChainReader
	node.Consensus = newCon
	node.Syncer = newSyncer
	newSigGetter := mthdsigapi.NewGetter(newChainReader)
	newMsgWaiter := message.NewWaiter(newChainReader, node.Blockstore, node.CborStore())
	newMsgSender := message.NewSender(node.Repo, node.Wallet, node.ChainReader, node.MsgPool, node.PubSub.Publish)
	node.PlumbingAPI = api2impl.New(newSigGetter, newMsgSender, newMsgWaiter)

	defaultSenderGetter := func() (address.Address, error) {
		return message.GetAndMaybeSetDefaultSenderAddress(node.Repo, node.Wallet)
	}
	node.lookup = lookup.NewChainLookupService(newChainReader, defaultSenderGetter, node.Blockstore)
	return nil
}

// PeerKeys are a list of keys for peers that can be used in testing.
var PeerKeys = []crypto.PrivKey{
	mustGenKey(101),
	mustGenKey(102),
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

// GenNode allows you to completely configure a node for testing.
func GenNode(t *testing.T, tno *TestNodeOptions) *Node {
	r := repo.NewInMemoryRepo()
	r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"
	if !tno.OfflineMode {
		r.Config().Swarm.Address = "/ip4/127.0.0.1/tcp/0"
	}
	// set a random port here so things don't break in the event we make
	// a parallel request
	// TODO: can we use port 0 yet?
	port, err := testhelpers.GetFreePort()
	require.NoError(t, err)
	r.Config().API.Address = fmt.Sprintf(":%d", port)

	// This needs to preserved to keep the test runtime (and corresponding timeouts) sane
	err = os.Setenv("FIL_USE_SMALL_SECTORS", "true")
	require.NoError(t, err)
	err = Init(context.Background(), r, tno.GenesisFunc, tno.InitOpts...)
	require.NoError(t, err)

	localCfgOpts, err := OptionsFromRepo(r)
	require.NoError(t, err)

	localCfgOpts = append(localCfgOpts, configOptions...)

	// enables or disables libp2p
	localCfgOpts = append(localCfgOpts, func(c *Config) error {
		c.OfflineMode = tno.OfflineMode
		return nil
	})

	nd, err := New(context.Background(), localCfgOpts...)

	require.NoError(t, err)
	return nd
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
