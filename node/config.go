package node

import (
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
)

// OptionsFromRepo takes a repo and returns options that configure a node node
// to use the given repo.
func OptionsFromRepo(r repo.Repo) []ConfigOpt {
	cfgopts := optionsFromConfig(r.Config())

	dsopt := func(c *Config) error {
		c.Datastore = r.Datastore()
		return nil
	}

	return append(cfgopts, dsopt)
}

func optionsFromConfig(cfg *config.Config) []ConfigOpt {
	return nil
}
