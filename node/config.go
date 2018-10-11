package node

import (
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	libp2p "gx/ipfs/QmVM6VuGaWcAaYjxG2om6XxMmpP3Rt9rw4nbMXVNYAPLhS/go-libp2p"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/repo"
)

// OptionsFromRepo takes a repo and returns options that configure a node node
// to use the given repo.
func OptionsFromRepo(r repo.Repo) ([]ConfigOpt, error) {
	sk, err := privKeyFromKeystore(r)
	if err != nil {
		return nil, err
	}
	cfg := r.Config()
	cfgopts := []ConfigOpt{
		// Libp2pOptions can only be called once, so add all options here
		Libp2pOptions(
			libp2p.ListenAddrStrings(cfg.Swarm.Address),
			libp2p.Identity(sk),
		),
	}

	dsopt := func(c *Config) error {
		c.Repo = r
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
