package node

import (
	"github.com/filecoin-project/go-filecoin/config"
)

// OptionsFromConfig takes a config file and returns the necessary
// configuration options to ensure that a constructed node is built as
// specified by the config file.
func OptionsFromConfig(cfg *config.Config) []ConfigOpt {
	return nil
}
