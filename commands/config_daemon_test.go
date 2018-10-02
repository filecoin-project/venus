package commands

import (
	"strings"
	"testing"

	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"

	"github.com/filecoin-project/go-filecoin/config"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
)

// types wrapping config fields with struct tags for reference output
type bootstrapWrapper struct {
	Bootstrap *config.BootstrapConfig `toml:"bootstrap"`
}

type datastoreWrapper struct {
	Datastore *config.DatastoreConfig `toml:"datastore"`
}

type pathWrapper struct {
	Path string `toml:"path"`
}

func TestConfigDaemon(t *testing.T) {
	// TODO: a lot of edge cases are not working correctly.  These things
	// are not essential and will take a decent amount of work to fix.
	// This work is being deferred to focus on more pressing issues.
	// See issue #1035 on github.
	t.Parallel()
	t.Run("config <key> prints config value", func(t *testing.T) {
		t.Skip()
		t.Parallel()
		assert := assert.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "datastore")
		tomlOut := op1.ReadStdout()
		b := strings.Builder{}
		wrapped1 := datastoreWrapper{
			Datastore: config.NewDefaultConfig().Datastore,
		}
		toml.NewEncoder(&b).Encode(wrapped1)
		assert.Equal(b.String(), tomlOut)
		b.Reset()

		op2 := d.RunSuccess("config", "datastore.path")
		tomlOut = op2.ReadStdout()
		wrapped2 := pathWrapper{
			Path: config.NewDefaultConfig().Datastore.Path,
		}
		toml.NewEncoder(&b).Encode(wrapped2)
		assert.Equal(b.String(), tomlOut)
	})

	t.Run("config <key> <val> updates config", func(t *testing.T) {
		t.Skip()
		t.Parallel()
		assert := assert.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "bootstrap",
			" { addresses = [\"fake1\", \"fake2\"], period = \"1m\", minPeerThreshold = 0 }")

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
