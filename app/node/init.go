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
	initImports []*crypto.KeyInfo
}

// InitOpt is an option for initialization of a node's repo.
type InitOpt func(*initCfg)

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

	_, err = createPeerKey(r.Keystore())
	if err != nil {
		return errors.Wrap(err, "Could not Create P2p key")
	}

	if err = r.ReplaceConfig(r.Config()); err != nil {
		return errors.Wrap(err, "failed to write config")
	}
	return nil
}

func createPeerKey(store fskeystore.Keystore) (acrypto.PrivKey, error) {
	var err error
	pk, _, err := acrypto.GenerateKeyPair(acrypto.RSA, defaultPeerKeyBits)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create peer key")
	}

	kbytes, err := acrypto.MarshalPrivateKey(pk)
	if err != nil {
		return nil, err
	}

	if err := store.Put("self", kbytes); err != nil {
		return nil, errors.Wrap(err, "failed to store private key")
	}
	return pk, nil
}
