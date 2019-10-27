package node

import (
	libp2p "github.com/libp2p/go-libp2p"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	errors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
)

// OptionsFromRepo takes a repo and returns options that configure a node
// to use the given repo.
func OptionsFromRepo(r repo.Repo) ([]BuilderOpt, error) {
	sk, err := privKeyFromKeystore(r)
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

func privKeyFromKeystore(r repo.Repo) (ci.PrivKey, error) {
	sk, err := r.Keystore().Get("self")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get key from keystore")
	}

	return sk, nil
}
