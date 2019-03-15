package node

import (
	"context"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	offline "gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	ci "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var ErrLittleBits = errors.New("Bitsize less than 1024 is considered unsafe") // nolint: golint

// InitCfg contains configuration for initializing a node
type InitCfg struct {
	PeerKey                 ci.PrivKey
	DefaultWalletAddress    address.Address
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

// AutoSealIntervalSecondsOpt configures the daemon to check for and seal any staged sectors on an interval.
func AutoSealIntervalSecondsOpt(autoSealIntervalSeconds uint) InitOpt {
	return func(c *InitCfg) {
		c.AutoSealIntervalSeconds = autoSealIntervalSeconds
	}
}

// Init initializes a filecoin node in the given repo.
func Init(ctx context.Context, r repo.Repo, gen consensus.GenesisInitFunc, opts ...InitOpt) error {
	cfg := new(InitCfg)
	for _, o := range opts {
		o(cfg)
	}

	bs := bstore.NewBlockstore(r.Datastore())
	cst := &hamt.CborIpldStore{Blocks: bserv.New(bs, offline.Exchange(bs))}

	if _, err := chain.Init(ctx, r, bs, cst, gen); err != nil {
		return errors.Wrap(err, "Could not Init Node")
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

	newConfig.Mining.AutoSealIntervalSeconds = cfg.AutoSealIntervalSeconds

	if cfg.DefaultWalletAddress != (address.Undef) {
		newConfig.Wallet.DefaultAddress = cfg.DefaultWalletAddress
	} else if r.Config().Wallet.DefaultAddress == (address.Undef) {
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
		return address.Undef, errors.Wrap(err, "failed to set up wallet backend")
	}

	addr, err := backend.NewAddress()
	if err != nil {
		return address.Undef, errors.Wrap(err, "failed to create address")
	}

	return addr, err
}
