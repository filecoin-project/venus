package cfg

import (
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/repo"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"testing"
)

func TestConfigGet(t *testing.T) {
	t.Parallel()
	t.Run("emits the referenced config value", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		require := require.New(t)

		repo := repo.NewInMemoryRepo()
		cfgAPI := NewConfig(repo)

		out, err := cfgAPI.Get("bootstrap")

		require.NoError(err)
		expected := config.NewDefaultConfig().Bootstrap
		assert.Equal(expected, out)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		repo := repo.NewInMemoryRepo()
		cfgAPI := NewConfig(repo)

		_, err := cfgAPI.Get("nonexistantkey")
		assert.EqualError(err, "key: nonexistantkey invalid for config")

		_, err = cfgAPI.Get("bootstrap.nope")
		assert.EqualError(err, "key: bootstrap.nope invalid for config")

		_, err = cfgAPI.Get(".inval.id-key")
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

		repo := repo.NewInMemoryRepo()
		cfgAPI := NewConfig(repo)

		jsonBlob := `{"addresses": ["bootup1", "bootup2"]}`

		err := cfgAPI.Set("bootstrap", jsonBlob)
		require.NoError(err)
		out, err := cfgAPI.Get("bootstrap")
		require.NoError(err)

		// validate output
		expected := config.NewDefaultConfig().Bootstrap
		expected.Addresses = []string{"bootup1", "bootup2"}
		assert.Equal(expected, out)

		// validate config write
		cfg := repo.Config()
		assert.Equal(expected, cfg.Bootstrap)
		assert.Equal(defaultCfg.Datastore, cfg.Datastore)

		err = cfgAPI.Set("api.address", ":1234")
		require.NoError(err)
		assert.Equal(":1234", cfg.API.Address)

		testAddr := address.TestAddress2.String()
		err = cfgAPI.Set("mining.minerAddress", testAddr)
		require.NoError(err)
		assert.Equal(testAddr, cfg.Mining.MinerAddress.String())

		err = cfgAPI.Set("wallet.defaultAddress", testAddr)
		require.NoError(err)
		assert.Equal(testAddr, cfg.Wallet.DefaultAddress.String())

		testSwarmAddr := "/ip4/0.0.0.0/tcp/0"
		err = cfgAPI.Set("swarm.address", testSwarmAddr)
		require.NoError(err)
		assert.Equal(testSwarmAddr, cfg.Swarm.Address)

		err = cfgAPI.Set("heartbeat.nickname", "Nickleless")
		require.NoError(err)
		assert.Equal("Nickleless", cfg.Heartbeat.Nickname)

		err = cfgAPI.Set("datastore.path", "/dev/null")
		require.NoError(err)
		assert.Equal("/dev/null", cfg.Datastore.Path)
	})

	t.Run("failure cases fail", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		repo := repo.NewInMemoryRepo()
		cfgAPI := NewConfig(repo)

		// bad key
		jsonBlob := `{"addresses": ["bootup1", "bootup2"]}`

		err := cfgAPI.Set("botstrap", jsonBlob)
		assert.EqualError(err, "json: unknown field \"botstrap\"")

		// bad value type (bootstrap is a struct not a list)
		jsonBlobBadType := `["bootup1", "bootup2"]`
		err = cfgAPI.Set("bootstrap", jsonBlobBadType)
		assert.Error(err)

		// bad JSON
		jsonBlobInvalid := `{"addresses": [bootup1, "bootup2"]}`

		err = cfgAPI.Set("bootstrap", jsonBlobInvalid)
		assert.EqualError(err, "json: cannot unmarshal string into Go struct field Config.bootstrap of type config.BootstrapConfig")

		// bad address
		jsonBlobBadAddr := "f4cqnyc0muxjajygqavu645m8ja04vckk2kcorrupt"
		err = cfgAPI.Set("wallet.defaultAddress", jsonBlobBadAddr)
		assert.EqualError(err, address.ErrUnknownProtocol.Error())
	})

	t.Run("validates the node nickname", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)

		repo := repo.NewInMemoryRepo()
		cfgAPI := NewConfig(repo)

		err := cfgAPI.Set("heartbeat.nickname", "Bad Nickname")

		assert.EqualError(err, `"heartbeat.nickname" must only contain letters`)
	})
}
