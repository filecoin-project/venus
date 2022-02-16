package node

import (
	libp2p "github.com/libp2p/go-libp2p"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	errors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/repo"
)

// OptionsFromRepo takes a repo and returns options that configure a node
// to use the given repo.
func OptionsFromRepo(r repo.Repo) ([]BuilderOpt, error) {
	sk, err := loadPrivKeyFromKeystore(r)
	if err != nil {
		return nil, err
	}

	cfg := r.Config()
	cfgopts := []BuilderOpt{
		// Libp2pOptions can only be called once, so add all options here.
		Libp2pOptions(
			libp2p.ListenAddrStrings(cfg.Swarm.Address),
			libp2p.Identity(sk),
		),
	}

	dsopt := func(c *Builder) error {
		c.repo = r
		return nil
	}

	return append(cfgopts, dsopt), nil
}

func loadPrivKeyFromKeystore(r repo.Repo) (ci.PrivKey, error) {
	data, err := r.Keystore().Get("self")
	if err != nil {
		if err.Error() == "no key by the given name was found" {
			return createPeerKey(r.Keystore())
		}
		return nil, errors.Wrap(err, "failed to get key from keystore")
	}
	sk, err := ci.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal private key failed")
	}
	return sk, nil
}

/*func initPeerKey(store fskeystore.Keystore, pk acrypto.PrivKey) error {
	var err error
	if pk == nil {
		pk, _, err = acrypto.GenerateKeyPair(acrypto.RSA, defaultPeerKeyBits)
		if err != nil {
			return errors.Wrap(err, "failed to create peer key")
		}
	}

	kbytes, err := acrypto.MarshalPrivateKey(pk)
	if err != nil {
		return err
	}

	if err := store.Put("self", kbytes); err != nil {
		return errors.Wrap(err, "failed to store private key")
	}
	return nil
}*/
