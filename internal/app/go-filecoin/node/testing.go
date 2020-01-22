package node

import (
	"context"
	"math/rand"
	"testing"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
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

// MakeChainSeed creates a chain seed struct (see above) from a given
// genesis config
func MakeChainSeed(t *testing.T, cfg *gengen.GenesisCfg) *ChainSeed {
	t.Helper()

	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	cst := hamt.CSTFromBstore(bstore)

	info, err := gengen.GenGen(context.TODO(), cfg, cst, bstore)
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

// ConfigOpt mutates a node config post initialization
type ConfigOpt func(*config.Config)

// MinerConfigOpt is a config option that sets a node's miner address to one of
// the chain seed's miner addresses
func (cs *ChainSeed) MinerConfigOpt(which int) ConfigOpt {
	return func(cfg *config.Config) {
		m := cs.info.Miners[which]
		cfg.Mining.MinerAddress = m.Address
	}
}

// KeyInitOpt is a node init option that imports one of the chain seed's
// keys to a node's wallet
func (cs *ChainSeed) KeyInitOpt(which int) InitOpt {
	kinfo := cs.info.Keys[which]
	return ImportKeyOpt(kinfo)
}

// FixtureChainSeed returns the genesis function that
func FixtureChainSeed(t *testing.T) *ChainSeed {
	return MakeChainSeed(t, &fixtures.TestGenGenConfig)
}

// DefaultAddressConfigOpt is a node config option setting the default address
func DefaultAddressConfigOpt(addr address.Address) ConfigOpt {
	return func(cfg *config.Config) {
		cfg.Wallet.DefaultAddress = addr
	}
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

// FakeProofVerifierBuilderOpts returns default configuration for testing
func FakeProofVerifierBuilderOpts() []BuilderOpt {
	return []BuilderOpt{
		VerifierConfigOption(&verification.FakeVerifier{
			VerifyPoStValid: true,
			VerifySealValid: true,
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
