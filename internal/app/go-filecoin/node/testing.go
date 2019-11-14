package node

import (
	"context"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
	gengen "github.com/filecoin-project/go-filecoin/tools/gengen/util"
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
	BuilderOpts []BuilderOpt
	GenesisFunc consensus.GenesisInitFunc
	InitOpts    []InitOpt
	Seed        *ChainSeed
}

// MakeChainSeed creates a chain seed struct (see above) from a given
// genesis config
func MakeChainSeed(t *testing.T, cfg *gengen.GenesisCfg) *ChainSeed {
	t.Helper()

	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	cst := hamt.CSTFromBstore(bstore)
	genesisTime := time.Unix(123456789, 0)

	info, err := gengen.GenGen(context.TODO(), cfg, cst, bstore, 0, genesisTime)
	require.NoError(t, err)

	return &ChainSeed{
		info:   info,
		cst:    cst,
		bstore: bstore,
	}
}

// GenesisInitFunc is a th.GenesisInitFunc using the chain seed
func (cs *ChainSeed) GenesisInitFunc(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*block.Block, error) {
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

	var blk block.Block
	if err := cst.Get(context.TODO(), cs.info.GenesisCid, &blk); err != nil {
		return nil, err
	}

	return &blk, nil
}

// GiveKey gives the given key to the given node
func (cs *ChainSeed) GiveKey(t *testing.T, nd *Node, key int) address.Address {
	t.Helper()
	bcks := nd.Wallet.Wallet.Backends(wallet.DSBackendType)
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
func MakeNodeWithChainSeed(t *testing.T, seed *ChainSeed, builderopts []BuilderOpt, initopts ...InitOpt) *Node { // nolint: golint
	t.Helper()
	tno := TestNodeOptions{OfflineMode: false, GenesisFunc: seed.GenesisInitFunc, BuilderOpts: builderopts, InitOpts: initopts}
	return GenNode(t, &tno)
}

// ConnectNodes connects two nodes together
func ConnectNodes(t *testing.T, a, b *Node) {
	t.Helper()
	pi := peer.AddrInfo{
		ID:    b.Host().ID(),
		Addrs: b.Host().Addrs(),
	}

	err := a.Host().Connect(context.TODO(), pi)
	if err != nil {
		t.Fatal(err)
	}
}

// MakeNodesUnstartedWithGif creates some new nodes with an InMemoryRepo and fake proof verifier.
// The repo is initialized with a supplied genesis init function.
// Call StartNodes to start them.
func MakeNodesUnstartedWithGif(t *testing.T, numNodes int, offlineMode bool, gif consensus.GenesisInitFunc) []*Node {
	tno := TestNodeOptions{
		OfflineMode: offlineMode,
		GenesisFunc: gif,
		BuilderOpts: DefaultTestingConfig(),
	}

	var out []*Node
	for i := 0; i < numNodes; i++ {
		nd := GenNode(t, &tno)
		out = append(out, nd)
	}

	return out
}

// MakeNodesUnstarted creates some new nodes with an InMemoryRepo, fake proof verifier, and default genesis block.
// Call StartNodes to start them.
func MakeNodesUnstarted(t *testing.T, numNodes int, offlineMode bool) []*Node {
	return MakeNodesUnstartedWithGif(t, numNodes, offlineMode, th.DefaultGenesis)
}

// MakeOfflineNode returns a single unstarted offline node.
func MakeOfflineNode(t *testing.T) *Node {
	return MakeNodesUnstartedWithGif(t,
		1,    /* 1 node */
		true, /* offline */
		th.DefaultGenesis)[0]
}

// DefaultTestingConfig returns default configuration for testing
func DefaultTestingConfig() []BuilderOpt {
	return []BuilderOpt{
		VerifierConfigOption(&verification.FakeVerifier{
			VerifyPoStValid:                true,
			VerifyPieceInclusionProofValid: true,
			VerifySealValid:                true,
		}),
	}
}

// StartNodes starts some nodes, failing on any error.
func StartNodes(t *testing.T, nds []*Node) {
	t.Helper()
	for _, nd := range nds {
		if err := nd.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

// StopNodes initiates shutdown of some nodes.
func StopNodes(nds []*Node) {
	for _, nd := range nds {
		nd.Stop(context.Background())
	}
}

// MustCreateStorageMinerResult contains the result of a CreateStorageMiner command
type MustCreateStorageMinerResult struct {
	MinerAddress *address.Address
	Err          error
}

// PeerKeys are a list of keys for peers that can be used in testing.
var PeerKeys = []crypto.PrivKey{
	mustGenKey(101),
	mustGenKey(102),
}

// TestGenCfg is a genesis configuration used for tests.
var TestGenCfg = &gengen.GenesisCfg{
	ProofsMode: types.TestProofsMode,
	Keys:       2,
	Miners: []*gengen.CreateStorageMinerConfig{
		{
			Owner:               0,
			NumCommittedSectors: 100,
			PeerID:              mustPeerID(PeerKeys[0]).Pretty(),
			SectorSize:          types.OneKiBSectorSize.Uint64(),
		},
	},
	Network: "go-filecoin-test",
	PreAlloc: []string{
		"10000",
		"10000",
	},
}

// GenNode allows you to completely configure a node for testing.
func GenNode(t *testing.T, tno *TestNodeOptions) *Node {
	r := repo.NewInMemoryRepo()
	sectorDir, err := ioutil.TempDir("", "go-fil-test-sectors")
	require.NoError(t, err)

	if tno.GenesisFunc != nil {
		err = Init(context.Background(), r, tno.GenesisFunc, tno.InitOpts...)
	} else {
		err = Init(context.Background(), r, tno.Seed.GenesisInitFunc, tno.InitOpts...)
	}
	require.NoError(t, err)

	r.Config().SectorBase.RootDir = sectorDir
	r.Config().Swarm.Address = "/ip4/0.0.0.0/tcp/0"
	if !tno.OfflineMode {
		r.Config().Swarm.Address = "/ip4/127.0.0.1/tcp/0"
	}

	localCfgOpts, err := OptionsFromRepo(r)
	require.NoError(t, err)

	localCfgOpts = append(localCfgOpts, tno.BuilderOpts...)

	// enables or disables libp2p
	localCfgOpts = append(localCfgOpts, func(c *Builder) error {
		c.offlineMode = tno.OfflineMode
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
