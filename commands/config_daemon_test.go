package commands

import (
	"strings"
	"testing"

	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"

	"github.com/filecoin-project/go-filecoin/config"

	"github.com/stretchr/testify/assert"
)

// types wrapping config fields with struct tags for reference output
type bootstrapWrapper struct {
	Bootstrap *config.BootstrapConfig `toml:"bootstrap"`
}

type addressesWrapper struct {
	Addresses []string `toml:"addresses"`
}

func TestConfigDaemon(t *testing.T) {
	t.Run("config <key> prints config value", func(t *testing.T) {
		assert := assert.New(t)

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "bootstrap")
		tomlOut := op1.ReadStdout()
		b := strings.Builder{}
		wrapped1 := bootstrapWrapper{
			Bootstrap: config.NewDefaultConfig().Bootstrap,
		}
		toml.NewEncoder(&b).Encode(wrapped1)
		assert.Equal(b.String(), tomlOut)
		b.Reset()

		op2 := d.RunSuccess("config", "bootstrap.addresses")
		tomlOut = op2.ReadStdout()
		wrapped2 := addressesWrapper{
			Addresses: config.NewDefaultConfig().Bootstrap.Addresses,
		}
		toml.NewEncoder(&b).Encode(wrapped2)
		assert.Equal(b.String(), tomlOut)
	})

	t.Run("config <key> <val> updates config", func(t *testing.T) {
		assert := assert.New(t)

		d := NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "bootstrap",
			" { addresses = [\"fake1\", \"fake2\"] }")

		// validate output
		tomlOut := op1.ReadStdout()
		b := strings.Builder{}
		wrapped := bootstrapWrapper{
			Bootstrap: config.NewDefaultConfig().Bootstrap,
		}
		wrapped.Bootstrap.Addresses = []string{"fake1", "fake2"}
		toml.NewEncoder(&b).Encode(wrapped)
		assert.Equal(b.String(), tomlOut)

		// validate config write
		cfg := d.Config()
		assert.Equal(cfg.Bootstrap, wrapped.Bootstrap)
	})
}
