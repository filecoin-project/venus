package commands

import (
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	th "github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/go-yaml/yaml"
)

// types wrapping config fields with struct tags for reference output
type bootstrapWrapper struct {
	Bootstrap *config.BootstrapConfig `yaml:"bootstrap"`
}

type datastoreWrapper struct {
	Datastore *config.DatastoreConfig `yaml:"datastore"`
}

type pathWrapper struct {
	Path string `yaml:"path"`
}

func TestConfigDaemon(t *testing.T) {
	// TODO: a lot of edge cases are not working correctly.  These things
	// are not essential and will take a decent amount of work to fix.
	// This work is being deferred to focus on more pressing issues.
	// See issue #1035 on github.
	t.Parallel()
	t.Run("config <key> prints config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "datastore")
		yamlOut := op1.ReadStdout()
		wrapped1 := datastoreWrapper{
			Datastore: config.NewDefaultConfig().Datastore,
		}
		decodedOutput1 := datastoreWrapper{}
		err := yaml.Unmarshal(yamlOut, &decodedOutput1)
		require.NoError(err)
		assert.Equal(wrapped1, decodedOutput1)

		op2 := d.RunSuccess("config", "datastore.path")
		yamlOut = op2.ReadStdout()
		wrapped2 := pathWrapper{
			Path: config.NewDefaultConfig().Datastore.Path,
		}
		decodedOutput2 := pathWrapper{}
		err = yaml.Unmarshal(yamlOut, &decodedOutput2)
		require.NoError(err)
		assert.Equal(wrapped2, decodedOutput2)
	})

	t.Run("config <key> <val> updates config", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		d := th.NewDaemon(t).Start()
		defer d.ShutdownSuccess()

		op1 := d.RunSuccess("config", "bootstrap", "addresses: [\"fake1\", \"fake2\"], period: \"1m\", minPeerThreshold: 0 }")

		// validate output
		yamlOut := op1.ReadStdout()
		b := strings.Builder{}
		wrapped := bootstrapWrapper{
			Bootstrap: config.NewDefaultConfig().Bootstrap,
		}
		wrapped.Bootstrap.Addresses = []string{"fake1", "fake2"}
		b, err := yaml.Marshal(wrapped)
		expected := strings.Replace(b, "0", "0.0", -1)
		assert.Equal(expected, yamlOut)

		// validate config write
		cfg := d.Config()
		assert.Equal(cfg.Bootstrap, wrapped.Bootstrap)
	})
}
