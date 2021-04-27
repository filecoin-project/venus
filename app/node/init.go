package node

import (
	"context"

	"github.com/filecoin-project/venus/pkg/repo/fskeystore"

	cbor "github.com/ipfs/go-ipld-cbor"
	acrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/genesis"
	"github.com/filecoin-project/venus/pkg/repo"
)

const defaultPeerKeyBits = 2048

// initCfg contains configuration for initializing a node's repo.
type initCfg struct {
	peerKey     acrypto.PrivKey
	defaultKey  *crypto.KeyInfo
	initImports []*crypto.KeyInfo
}

// InitOpt is an option for initialization of a node's repo.
type InitOpt func(*initCfg)

// PeerKeyOpt sets the private key for a node's 'self' libp2p identity.
// If unspecified, initialization will create a new one.
func PeerKeyOpt(k acrypto.PrivKey) InitOpt {
	return func(opts *initCfg) {
		opts.peerKey = k
	}
}

// DefaultKeyOpt sets the private key for the wallet's default account.
// If unspecified, initialization will create a new one.
func DefaultKeyOpt(ki *crypto.KeyInfo) InitOpt {
	return func(opts *initCfg) {
		opts.defaultKey = ki
	}
}

// ImportKeyOpt imports the provided key during initialization.
func ImportKeyOpt(ki *crypto.KeyInfo) InitOpt {
	return func(opts *initCfg) {
		opts.initImports = append(opts.initImports, ki)
	}
}

// Init initializes a Filecoin repo with genesis state and keys.
// This will always set the configuration for wallet default address (to the specified default
// key or a newly generated one), but otherwise leave the repo's config object intact.
// Make further configuration changes after initialization.
func Init(ctx context.Context, r repo.Repo, gen genesis.InitFunc, opts ...InitOpt) error {
	cfg := new(initCfg)
	for _, o := range opts {
		o(cfg)
	}

	bs := r.Datastore()
	cst := cbor.NewCborStore(bs)
	_, err := genesis.Init(ctx, r, bs, cst, gen)
	if err != nil {
		return errors.Wrap(err, "Could not Init Node")
	}

	if err := initPeerKey(r.Keystore(), cfg.peerKey); err != nil {
		return err
	}

	if err = r.ReplaceConfig(r.Config()); err != nil {
		return errors.Wrap(err, "failed to write config")
	}

	return nil
}

func initPeerKey(store fskeystore.Keystore, key acrypto.PrivKey) error {
	var err error
	if key == nil {
		key, _, err = acrypto.GenerateKeyPair(acrypto.RSA, defaultPeerKeyBits)
		if err != nil {
			return errors.Wrap(err, "failed to create peer key")
		}
	}
	data, err := key.Bytes()
	if err != nil {
		return err
	}
	if err := store.Put("self", data); err != nil {
		return errors.Wrap(err, "failed to store private key")
	}
	return nil
}
