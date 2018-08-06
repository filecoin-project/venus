package commands

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/node_api"
	"github.com/filecoin-project/go-filecoin/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigGet(t *testing.T) {
	t.Parallel()
	t.Run("emits the referenced config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		out, err := testhelpers.RunCommand(configCmd,
			[]string{"bootstrap"}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		require.NoError(err)
		b := strings.Builder{}
		wrapped := bootstrapWrapper{
			Bootstrap: config.NewDefaultConfig().Bootstrap,
		}
		toml.NewEncoder(&b).Encode(wrapped)
		assert.Equal(b.String(), out.Raw)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		ctx := context.Background()
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]
		res, err := testhelpers.RunCommand(configCmd,
			[]string{"nonexistantkey"}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "key: nonexistantkey invalid for config")

		res, err = testhelpers.RunCommand(configCmd,
			[]string{"bootstrap.nope"}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "key: bootstrap.nope invalid for config")

		res, err = testhelpers.RunCommand(configCmd,
			[]string{".inval.id-key."}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "key: .inval.id-key. invalid for config")
	})
}

func TestConfigSet(t *testing.T) {
	t.Parallel()
	t.Run("sets the config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		ctx := context.Background()
		defaultCfg := config.NewDefaultConfig()

		n := node.MakeNodesUnstarted(t, 1, true, true, func(c *node.Config) error {
			c.Repo.Config().API.Address = defaultCfg.API.Address
			return nil
		})[0]
		tomlBlob := `{addresses = ["bootup1", "bootup2"]}  `

		out, err := testhelpers.RunCommand(configCmd,
			[]string{"bootstrap", tomlBlob}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		require.NoError(err)

		// validate output
		b := strings.Builder{}
		wrapped := bootstrapWrapper{
			Bootstrap: config.NewDefaultConfig().Bootstrap,
		}
		wrapped.Bootstrap.Addresses = []string{"bootup1", "bootup2"}
		toml.NewEncoder(&b).Encode(wrapped)
		assert.Equal(b.String(), out.Raw)

		// validate config write
		cfg := n.Repo.Config()
		defaultCfg.Mining.RewardAddress = n.RewardAddress()
		assert.Equal(wrapped.Bootstrap, cfg.Bootstrap)
		assert.Equal(defaultCfg.API, cfg.API)
		assert.Equal(defaultCfg.Datastore, cfg.Datastore)
		assert.Equal(defaultCfg.Mining, cfg.Mining)
		assert.Equal(defaultCfg.Swarm, cfg.Swarm)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		ctx := context.Background()
		n := node.MakeNodesUnstarted(t, 1, true, true)[0]

		// bad key
		tomlBlob := `{addresses = ["bootup1", "bootup2"]}  `
		res, err := testhelpers.RunCommand(configCmd,
			[]string{"botstrap", tomlBlob}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "key: botstrap invalid for config")

		// bad value type (bootstrap is a struct not a list)
		tomlBlobBadType := `["bootup1", "bootup2"]`
		res, err = testhelpers.RunCommand(configCmd,
			[]string{"bootstrap", tomlBlobBadType}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "input could not be marshaled")

		// bad TOML
		tomlBlobInvalid := `{addresses =[""bootup1", "bootup2"]`
		res, err = testhelpers.RunCommand(configCmd,
			[]string{"bootstrap", tomlBlobInvalid}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "input could not be marshaled")

		// bad address
		tomlBlobBadAddr := `"fcqnyc0muxjajygqavu645m8ja04vckk2kcorrupt"`
		res, err = testhelpers.RunCommand(configCmd,
			[]string{"mining.rewardAddress", tomlBlobBadAddr}, nil, &Env{
				ctx: ctx,
				api: node_api.NewAPI(n),
			})
		assert.NoError(err)
		assert.Contains(res.Raw, "input could not be marshaled")
	})
}

func TestConfigMakeKey(t *testing.T) {
	t.Parallel()
	t.Run("all of table key printed", func(t *testing.T) {
		t.Parallel()
		var testStruct config.DatastoreConfig
		var testStructPtr *config.DatastoreConfig
		var testStructSlice []config.DatastoreConfig

		key := "parent1.parent2.thisKey"
		assert := assert.New(t)

		outKey := makeKey(key, reflect.TypeOf(testStruct))
		assert.Equal(key, outKey)

		outKey = makeKey(key, reflect.TypeOf(testStructPtr))
		assert.Equal(key, outKey)

		outKey = makeKey(key, reflect.TypeOf(testStructSlice))
		assert.Equal(key, outKey)
	})

	t.Run("last substring of other keys printed", func(t *testing.T) {
		t.Parallel()
		var testInt int
		var testString string
		var testStringSlice []string

		key := "parent1.parent2.thisKey"
		last := "thisKey"
		assert := assert.New(t)

		outKey := makeKey(key, reflect.TypeOf(testInt))
		assert.Equal(last, outKey)

		outKey = makeKey(key, reflect.TypeOf(testString))
		assert.Equal(last, outKey)

		outKey = makeKey(key, reflect.TypeOf(testStringSlice))
		assert.Equal(last, outKey)
	})
}
