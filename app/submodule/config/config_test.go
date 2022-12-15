package config

import (
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/config"
	repo2 "github.com/filecoin-project/venus/pkg/repo"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestConfigGet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("emits the referenced config value", func(t *testing.T) {
		repo := repo2.NewInMemoryRepo()
		cfgAPI := NewConfigModule(repo)

		out, err := cfgAPI.Get("bootstrap")

		require.NoError(t, err)
		expected := config.NewDefaultConfig().Bootstrap
		assert.Equal(t, expected, out)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		repo := repo2.NewInMemoryRepo()
		cfgAPI := NewConfigModule(repo)

		_, err := cfgAPI.Get("nonexistantkey")
		assert.EqualError(t, err, "key: nonexistantkey invalid for config")

		_, err = cfgAPI.Get("bootstrap.nope")
		assert.EqualError(t, err, "key: bootstrap.nope invalid for config")

		_, err = cfgAPI.Get(".inval.id-key")
		assert.EqualError(t, err, "key: .inval.id-key invalid for config")
	})
}

func TestConfigSet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("sets the config value", func(t *testing.T) {
		defaultCfg := config.NewDefaultConfig()

		repo := repo2.NewInMemoryRepo()
		cfgAPI := NewConfigModule(repo)

		jsonBlob := `{"addresses": ["bootup1", "bootup2"]}`

		err := cfgAPI.Set("bootstrap", jsonBlob)
		require.NoError(t, err)
		out, err := cfgAPI.Get("bootstrap")
		require.NoError(t, err)

		// validate output
		expected := config.NewDefaultConfig().Bootstrap
		expected.Addresses = []string{"bootup1", "bootup2"}
		assert.Equal(t, expected, out)

		// validate config write
		cfg := repo.Config()
		assert.Equal(t, expected, cfg.Bootstrap)
		assert.Equal(t, defaultCfg.Datastore, cfg.Datastore)

		err = cfgAPI.Set("api.apiAddress", ":1234")
		require.NoError(t, err)
		assert.Equal(t, ":1234", cfg.API.APIAddress)

		testAddr := testhelpers.RequireIDAddress(t, 100).String()
		err = cfgAPI.Set("walletModule.defaultAddress", testAddr)
		require.NoError(t, err)
		assert.Equal(t, testAddr, cfg.Wallet.DefaultAddress.String())

		testSwarmAddr := "/ip4/0.0.0.0/tcp/0"
		err = cfgAPI.Set("swarm.address", testSwarmAddr)
		require.NoError(t, err)
		assert.Equal(t, testSwarmAddr, cfg.Swarm.Address)

		err = cfgAPI.Set("datastore.path", "/dev/null")
		require.NoError(t, err)
		assert.Equal(t, "/dev/null", cfg.Datastore.Path)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		repo := repo2.NewInMemoryRepo()
		cfgAPI := NewConfigModule(repo)

		// bad key
		jsonBlob := `{"addresses": ["bootup1", "bootup2"]}`

		err := cfgAPI.Set("botstrap", jsonBlob)
		assert.EqualError(t, err, "json: unknown field \"botstrap\"")

		// bad value type (bootstrap is a struct not a list)
		jsonBlobBadType := `["bootup1", "bootup2"]`
		err = cfgAPI.Set("bootstrap", jsonBlobBadType)
		assert.Error(t, err)

		// bad JSON
		jsonBlobInvalid := `{"addresses": [bootup1, "bootup2"]}`

		err = cfgAPI.Set("bootstrap", jsonBlobInvalid)
		assert.EqualError(t, err, "json: cannot unmarshal string into Go struct field Config.bootstrap of type config.BootstrapConfig")

		// bad address
		jsonBlobBadAddr := "f4cqnyc0muxjajygqavu645m8ja04vckk2kcorrupt"
		err = cfgAPI.Set("walletModule.defaultAddress", jsonBlobBadAddr)
		assert.EqualError(t, err, address.ErrInvalidPayload.Error())
	})
}
