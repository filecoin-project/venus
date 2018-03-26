package node

import (
	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
)

// OptionsFromRepo takes a repo and returns options that configure a node node
// to use the given repo.
func OptionsFromRepo(r repo.Repo) []ConfigOpt {
	cfgopts := optionsFromConfig(r.Config())

	dsopt := func(c *Config) error {
		c.Repo = r
		return nil
	}

	return append(cfgopts, dsopt)
}

func optionsFromConfig(cfg *config.Config) []ConfigOpt {
	return []ConfigOpt{
		Libp2pOptions(libp2p.ListenAddrStrings(cfg.Swarm.Address)),
	}
}
