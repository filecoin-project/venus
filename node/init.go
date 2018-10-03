package node

import (
	"context"
	"encoding/json"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	offline "gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var ErrLittleBits = errors.New("Bitsize less than 1024 is considered unsafe") // nolint: golint
var genesisKey = datastore.NewKey("/consensus/genesisCid")

// InitCfg contains configuration for initializing a node
type InitCfg struct {
	PeerKey                 ci.PrivKey
	DefaultWalletAddress    address.Address
	PerformRealProofs       bool
	AutoSealIntervalSeconds uint
}

// InitOpt is an init option function
type InitOpt func(*InitCfg)

// PeerKeyOpt sets the private key for the nodes 'self' key
// this is the key that is used for libp2p identity
func PeerKeyOpt(k ci.PrivKey) InitOpt {
	return func(c *InitCfg) {
		c.PeerKey = k
	}
}

// DefaultWalletAddressOpt returns a config option that sets the default wallet address to the given address.
func DefaultWalletAddressOpt(addr address.Address) InitOpt {
	return func(c *InitCfg) {
		c.DefaultWalletAddress = addr
	}
}

// PerformRealProofsOpt configures the daemon to run the real (slow) PoSt and PoRep operations against small sectors.
func PerformRealProofsOpt(performRealProofs bool) InitOpt {
	return func(c *InitCfg) {
		c.PerformRealProofs = performRealProofs
	}
}

// AutoSealIntervalSecondsOpt configures the daemon to check for and seal any staged sectors on an interval.
func AutoSealIntervalSecondsOpt(autoSealIntervalSeconds uint) InitOpt {
	return func(c *InitCfg) {
		c.AutoSealIntervalSeconds = autoSealIntervalSeconds
	}
}

// Init initializes a filecoin node in the given repo
// TODO: accept options?
//  - configurable genesis block
func Init(ctx context.Context, r repo.Repo, gen consensus.GenesisInitFunc, opts ...InitOpt) error {
	cfg := new(InitCfg)
	for _, o := range opts {
		o(cfg)
	}

	// TODO(ipfs): make the blockstore and blockservice have the same interfaces
	// so that this becomes less painful
	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}

	// TODO the following should be wrapped in the chain.Store or a sub
	// interface.
	chainStore := chain.NewDefaultStore(r.ChainDatastore(), cst, nil)
	// Generate the genesis tipset.
	genesis, err := gen(cst, bs)
	if err != nil {
		return err
	}
	genTipSet, err := consensus.NewTipSet(genesis)
	if err != nil {
		return errors.Wrap(err, "failed to generate genesis block")
	}
	// Persist the genesis tipset to the repo.
	genTsas := &chain.TipSetAndState{
		TipSet:          genTipSet,
		TipSetStateRoot: genesis.StateRoot,
	}
	if err = chainStore.PutTipSetAndState(ctx, genTsas); err != nil {
		return errors.Wrap(err, "failed to put genesis block in chain store")
	}
	if err = chainStore.SetHead(ctx, genTipSet); err != nil {
		return errors.Wrap(err, "failed to persist genesis block in chain store")
	}
	// Persist the genesis cid to the repo.
	val, err := json.Marshal(genesis.Cid())
	if err != nil {
		return errors.Wrap(err, "failed to marshal genesis cid")
	}
	if err = r.Datastore().Put(genesisKey, val); err != nil {
		return errors.Wrap(err, "failed to persist genesis cid")
	}

	if cfg.PeerKey == nil {
		// TODO: make size configurable
		peerKey, err := makePrivateKey(2048)
		if err != nil {
			return errors.Wrap(err, "failed to create nodes private key")
		}

		cfg.PeerKey = peerKey
	}

	if err := r.Keystore().Put("self", cfg.PeerKey); err != nil {
		return errors.Wrap(err, "failed to store private key")
	}

	newConfig := r.Config()

	newConfig.Mining.PerformRealProofs = cfg.PerformRealProofs
	newConfig.Mining.AutoSealIntervalSeconds = cfg.AutoSealIntervalSeconds

	if cfg.DefaultWalletAddress != (address.Address{}) {
		newConfig.Wallet.DefaultAddress = cfg.DefaultWalletAddress
	} else if r.Config().Wallet.DefaultAddress == (address.Address{}) {
		// TODO: but behind a config option if this should be generated
		addr, err := newAddress(r)
		if err != nil {
			return errors.Wrap(err, "failed to generate default address")
		}

		newConfig.Wallet.DefaultAddress = addr
	}

	if err := r.ReplaceConfig(newConfig); err != nil {
		return errors.Wrap(err, "failed to update config with new values")
	}

	return nil
}

// makePrivateKey generates a new private key, which is the basis for a libp2p identity.
// borrowed from go-ipfs: `repo/config/init.go`
func makePrivateKey(nbits int) (ci.PrivKey, error) {
	if nbits < 1024 {
		return nil, ErrLittleBits
	}

	// create a public private key pair
	sk, _, err := ci.GenerateKeyPair(ci.RSA, nbits)
	if err != nil {
		return nil, err
	}

	return sk, nil
}

// newAddress creates a new private-public keypair in the default wallet
// and returns the address for it.
func newAddress(r repo.Repo) (address.Address, error) {
	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	if err != nil {
		return address.Address{}, errors.Wrap(err, "failed to set up wallet backend")
	}

	addr, err := backend.NewAddress()
	if err != nil {
		return address.Address{}, errors.Wrap(err, "failed to create address")
	}

	return addr, err
}
