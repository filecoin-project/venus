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

		n := node.MakeOfflineNode(t)

		api := New(n)

		out, err := api.Config().Get("bootstrap")

		require.NoError(err)
		expected := config.NewDefaultConfig().Bootstrap
		assert.Equal(expected, out)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		n := node.MakeOfflineNode(t)
		api := New(n)

		_, err := api.Config().Get("nonexistantkey")
		assert.EqualError(err, "key: nonexistantkey invalid for config")

		_, err = api.Config().Get("bootstrap.nope")
		assert.EqualError(err, "key: bootstrap.nope invalid for config")

		_, err = api.Config().Get(".inval.id-key")
		assert.EqualError(err, "key: .inval.id-key invalid for config")
	})
}

func nodeConfigFunc() node.ConfigOpt {
	defaultCfg := config.NewDefaultConfig()
	return func(c *node.Config) error {
		c.Repo.Config().Mining.AutoSealIntervalSeconds = defaultCfg.Mining.AutoSealIntervalSeconds
		c.Repo.Config().API.Address = defaultCfg.API.Address // overwrite value set with testhelpers.GetFreePort()
		return nil
	}
}

func TestConfigSet(t *testing.T) {
	t.Parallel()
	t.Run("sets the config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		defaultCfg := config.NewDefaultConfig()

		n := node.MakeNodesUnstarted(t, 1, true, true, []node.ConfigOpt{nodeConfigFunc()})[0]
		api := New(n)
		jsonBlob := `{"addresses": ["bootup1", "bootup2"]}`

		err := api.Config().Set("bootstrap", jsonBlob)
		require.NoError(err)
		out, err := api.Config().Get("bootstrap")
		require.NoError(err)

		// validate output
		expected := config.NewDefaultConfig().Bootstrap
		expected.Addresses = []string{"bootup1", "bootup2"}
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

		n := node.MakeOfflineNode(t)
		api := New(n)

		// bad key
		jsonBlob := `{"addresses": ["bootup1", "bootup2"]}`

		err := api.Config().Set("botstrap", jsonBlob)
		assert.EqualError(err, "json: unknown field \"botstrap\"")

		// bad value type (bootstrap is a struct not a list)
		jsonBlobBadType := `["bootup1", "bootup2"]`
		err = api.Config().Set("bootstrap", jsonBlobBadType)
		assert.Error(err)

		// bad JSON
		jsonBlobInvalid := `{"addresses": [bootup1, "bootup2"]}`

		err = api.Config().Set("bootstrap", jsonBlobInvalid)
		assert.EqualError(err, "json: cannot unmarshal string into Go struct field Config.bootstrap of type config.BootstrapConfig")

		// bad address
		jsonBlobBadAddr := "fcqnyc0muxjajygqavu645m8ja04vckk2kcorrupt"
		err = api.Config().Set("wallet.defaultAddress", jsonBlobBadAddr)
		assert.EqualError(err, "invalid character")
	})

	t.Run("validates the node nickname", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		n := node.MakeOfflineNode(t)
		api := New(n)

		err := api.Config().Set("heartbeat.nickname", "Bad Nickname")

		assert.EqualError(err, `"heartbeat.nickname" must only contain letters`)
	})
}
