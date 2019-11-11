package node

import (
	"context"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
)

const defaultPeerKeyBits = 2048

// initCfg contains configuration for initializing a node's repo.
type initCfg struct {
	peerKey    crypto.PrivKey
	defaultKey *types.KeyInfo
}

// InitOpt is an option for initialization of a node's repo.
type InitOpt func(*initCfg)

// PeerKeyOpt sets the private key for a node's 'self' libp2p identity.
// If unspecified, initialization will create a new one.
func PeerKeyOpt(k crypto.PrivKey) InitOpt {
	return func(opts *initCfg) {
		opts.peerKey = k
	}
}

// DefaultKeyOpt sets the private key for the wallet's default account.
// If unspecified, initialization will create a new one.
func DefaultKeyOpt(ki *types.KeyInfo) InitOpt {
	return func(opts *initCfg) {
		opts.defaultKey = ki
	}
}

// Init initializes a Filecoin repo with genesis state and keys.
// This will always set the configuration for wallet default address (to the specified default
// key or a newly generated one), but otherwise leave the repo's config object intact.
// Make further configuration changes after initialization.
func Init(ctx context.Context, r repo.Repo, gen consensus.GenesisInitFunc, opts ...InitOpt) error {
	cfg := new(initCfg)
	for _, o := range opts {
		o(cfg)
	}

	bs := bstore.NewBlockstore(r.Datastore())
	cst := hamt.CSTFromBstore(bs)
	if _, err := chain.Init(ctx, r, bs, cst, gen); err != nil {
		return errors.Wrap(err, "Could not Init Node")
	}

	if err := initPeerKey(r.Keystore(), cfg.peerKey); err != nil {
		return err
	}

	defaultKey, err := initDefaultKey(r.WalletDatastore(), cfg.defaultKey)
	if err != nil {
		return err
	}

	defaultAddress, err := defaultKey.Address()
	if err != nil {
		return errors.Wrap(err, "failed to extract address from default key")
	}
	r.Config().Wallet.DefaultAddress = defaultAddress
	if err = r.ReplaceConfig(r.Config()); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	return nil
}

func initPeerKey(store keystore.Keystore, key crypto.PrivKey) error {
	var err error
	if key == nil {
		key, _, err = crypto.GenerateKeyPair(crypto.RSA, defaultPeerKeyBits)
		if err != nil {
			return errors.Wrap(err, "failed to create peer key")
		}
	}
	if err := store.Put("self", key); err != nil {
		return errors.Wrap(err, "failed to store private key")
	}
	return nil
}

func initDefaultKey(store repo.Datastore, key *types.KeyInfo) (*types.KeyInfo, error) {
	backend, err := wallet.NewDSBackend(store)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open wallet datastore")
	}
	w := wallet.New(backend)
	if key == nil {
		key, err = w.NewKeyInfo()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create default key")
		}
	} else {
		if _, err := w.Import(key); err != nil {
			return nil, errors.Wrap(err, "failed to import default key")
		}
	}
	return key, nil
}
