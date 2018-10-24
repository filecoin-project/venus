package impl

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/node"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigGet(t *testing.T) {
	t.Parallel()
	t.Run("emits the referenced config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		api := New(n)

		out, err := api.Config().Get("bootstrap")

		require.NoError(err)
		expected := config.NewDefaultConfig().Bootstrap
		assert.Equal(expected, out)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]
		api := New(n)

		_, err := api.Config().Get("nonexistantkey")
		assert.EqualError(err, "key: nonexistantkey invalid for config")

		_, err = api.Config().Get("bootstrap.nope")
		assert.EqualError(err, "key: bootstrap.nope invalid for config")

		_, err = api.Config().Get(".inval.id-key")
		assert.EqualError(err, "key: .inval.id-key invalid for config")
	})
}

func TestConfigSet(t *testing.T) {
	t.Parallel()
	t.Run("sets the config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		defaultCfg := config.NewDefaultConfig()

		n := node.MakeNodesUnstarted(t, 1, true, true, func(c *node.Config) error {
			c.Repo.Config().Mining.AutoSealIntervalSeconds = defaultCfg.Mining.AutoSealIntervalSeconds
			c.Repo.Config().Mining.PerformRealProofs = false     // overwrite value set with testhelpers.ensurePerformRealProofsDefaultsToTrue
			c.Repo.Config().API.Address = defaultCfg.API.Address // overwrite value set with testhelpers.GetFreePort()
			return nil
		})[0]
		api := New(n)
		tomlBlob := `{addresses = ["bootup1", "bootup2"]}  `

		out, err := api.Config().Set("bootstrap", tomlBlob)
		require.NoError(err)

		// validate output
		expected := config.NewDefaultConfig().Bootstrap
		expected.Addresses = []string{"bootup1", "bootup2"}
		expected.Period = ""
		assert.Equal(expected, out)

		// validate config write
		cfg := n.Repo.Config()
		assert.Equal(expected, cfg.Bootstrap)
		assert.Equal(defaultCfg.API, cfg.API)
		assert.Equal(defaultCfg.Datastore, cfg.Datastore)
		assert.Equal(defaultCfg.Mining, cfg.Mining)
		assert.Equal(&config.SwarmConfig{
			Address: "/ip4/0.0.0.0/tcp/0",
		}, cfg.Swarm) // default overwritten in node.MakeNodesUnstarted()
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		n := node.MakeNodesUnstarted(t, 1, true, true)[0]
		api := New(n)

		// bad key
		tomlBlob := `{addresses = ["bootup1", "bootup2"]}  `

		_, err := api.Config().Set("botstrap", tomlBlob)
		assert.EqualError(err, "key: botstrap invalid for config")

		// bad value type (bootstrap is a struct not a list)
		tomlBlobBadType := `["bootup1", "bootup2"]`
		_, err = api.Config().Set("bootstrap", tomlBlobBadType)
		assert.EqualError(err, "input could not be marshaled to sub-config at: bootstrap: Near line 2 (last key parsed 'bootstrap'): expected '.' or ']' to end table name, but got ',' instead")

		// bad TOML
		tomlBlobInvalid := `{addresses =[""bootup1", "bootup2"]`
		_, err = api.Config().Set("bootstrap", tomlBlobInvalid)
		assert.EqualError(err, "input could not be marshaled to sub-config at: bootstrap: Near line 1 (last key parsed 'bootstrap.addresses'): expected a comma or array terminator ']', but got 'b' instead")

		// bad address
		tomlBlobBadAddr := `"fcqnyc0muxjajygqavu645m8ja04vckk2kcorrupt"`
		_, err = api.Config().Set("wallet.defaultAddress", tomlBlobBadAddr)
		assert.EqualError(err, "input could not be marshaled to sub-config at: wallet.defaultAddress: invalid character")
	})

	t.Run("validates the node nickname", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		n := node.MakeNodesUnstarted(t, 1, true, true)[0]
		api := New(n)

		_, err := api.Config().Set("stats.nickname", "\"Bad Nickname\"")

		assert.EqualError(err, "node nickname must only contain letters")
	})
}
