package node

import (
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	libp2p "gx/ipfs/QmY51bqSM5XgxQZqsBrQcRkKTnCb8EKpJpR9K6Qax7Njco/go-libp2p"
	ci "gx/ipfs/Qme1knMqwt1hKZbc1BmQFmnm9f36nyQGwXxPGVpVJ9rMK5/go-libp2p-crypto"

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
