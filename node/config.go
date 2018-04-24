package node

import (
	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"

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
		RewardAddress(cfg.Mining.RewardAddress),
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
