package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestDefaults(t *testing.T) {
	tf.UnitTest(t)

	cfg := NewDefaultConfig()

	bs := []string{}
	assert.Equal(t, "/ip4/127.0.0.1/tcp/3453", cfg.API.Address)
	assert.Equal(t, "/ip4/0.0.0.0/tcp/6000", cfg.Swarm.Address)
	assert.Equal(t, bs, cfg.Bootstrap.Addresses)
}

func TestWriteFile(t *testing.T) {
	tf.UnitTest(t)

	dir, err := ioutil.TempDir("", "config")
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := NewDefaultConfig()

	assert.NoError(t, cfg.WriteFile(filepath.Join(dir, "config.json")))
	content, err := ioutil.ReadFile(filepath.Join(dir, "config.json"))
	assert.NoError(t, err)

	assert.Equal(t,
		`{
	"api": {
		"address": "/ip4/127.0.0.1/tcp/3453",
		"accessControlAllowOrigin": [
			"http://localhost:8080",
			"https://localhost:8080",
			"http://127.0.0.1:8080",
			"https://127.0.0.1:8080"
		],
		"accessControlAllowCredentials": false,
		"accessControlAllowMethods": [
			"GET",
			"POST",
			"PUT"
		]
	},
	"bootstrap": {
		"addresses": [],
		"minPeerThreshold": 0,
		"period": "1m"
	},
	"datastore": {
		"type": "badgerds",
		"path": "badger"
	},
	"heartbeat": {
		"beatTarget": "",
		"beatPeriod": "3s",
		"reconnectPeriod": "10s",
		"nickname": ""
	},
	"mining": {
		"minerAddress": "empty",
		"autoSealIntervalSeconds": 120,
		"storagePrice": "0"
	},
	"mpool": {
		"maxPoolSize": 10000,
		"maxNonceGap": "100"
	},
	"net": "",
	"observability": {
		"metrics": {
			"prometheusEnabled": false,
			"reportInterval": "5s",
			"prometheusEndpoint": "/ip4/0.0.0.0/tcp/9400"
		},
		"tracing": {
			"jaegerTracingEnabled": false,
			"probabilitySampler": 1,
			"jaegerEndpoint": "http://localhost:14268/api/traces"
		}
	},
	"sectorbase": {
		"rootdir": ""
	},
	"swarm": {
		"address": "/ip4/0.0.0.0/tcp/6000"
	},
	"wallet": {
		"defaultAddress": "empty"
	}
}`,
		string(content),
	)

	assert.NoError(t, os.Remove(filepath.Join(dir, "config.json")))
}

func TestSetRejectsInvalidNicks(t *testing.T) {
	tf.UnitTest(t)

	cfg := NewDefaultConfig()

	// sic: json includes the quotes in the value
	err := cfg.Set("heartbeat.nickname", "\"goodnick\"")
	assert.NoError(t, err)
	err = cfg.Set("heartbeat.nickname", "bad nick<p>")
	assert.Error(t, err)
	err = cfg.Set("heartbeat", `{"heartbeat": "bad nick"}`)
	assert.Error(t, err)
}

func TestConfigRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	dir, err := ioutil.TempDir("", "config")
	assert.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(dir))
	}()

	cfg := NewDefaultConfig()

	cfgpath := filepath.Join(dir, "config.json")
	assert.NoError(t, cfg.WriteFile(cfgpath))

	cfgout, err := ReadFile(cfgpath)
	assert.NoError(t, err)

	assert.Equal(t, cfg, cfgout)
}

func TestConfigReadFileDefaults(t *testing.T) {
	tf.UnitTest(t)

	t.Run("all sections exist", func(t *testing.T) {
		cfgpath, cleaner, err := createConfigFile(`
		{
			"api": {
				"address": "/ip4/127.0.0.1/tcp/9999",
				"keyThatDoesntExit": false
			},
			"swarm": {
				"keyThatDoesntExit": "hello"
			}
		}`)
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, cleaner())
		}()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(t, err)

		assert.Equal(t, cfg.API.Address, "/ip4/127.0.0.1/tcp/9999")
		assert.Equal(t, cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/6000")
	})

	t.Run("missing one section", func(t *testing.T) {
		cfgpath, cleaner, err := createConfigFile(`
		{
			"api": {
				"address": "/ip4/127.0.0.1/tcp/9999",
				"keyThatDoesntExit'": false
			}
		}`)
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, cleaner())
		}()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(t, err)

		assert.Equal(t, cfg.API.Address, "/ip4/127.0.0.1/tcp/9999")
		assert.Equal(t, cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/6000")
	})

	t.Run("empty file", func(t *testing.T) {
		cfgpath, cleaner, err := createConfigFile("")
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, cleaner())
		}()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(t, err)

		assert.Equal(t, cfg.API.Address, "/ip4/127.0.0.1/tcp/3453")
		assert.Equal(t, cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/6000")
	})
}

func TestConfigGet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("valid gets", func(t *testing.T) {
		cfg := NewDefaultConfig()

		out, err := cfg.Get("api.address")
		assert.NoError(t, err)
		assert.Equal(t, cfg.API.Address, out)

		out, err = cfg.Get("api.accessControlAllowOrigin")
		assert.NoError(t, err)
		assert.Equal(t, cfg.API.AccessControlAllowOrigin, out)

		out, err = cfg.Get("api")
		assert.NoError(t, err)
		assert.Equal(t, cfg.API, out)

		out, err = cfg.Get("bootstrap.addresses")
		assert.NoError(t, err)
		assert.Equal(t, cfg.Bootstrap.Addresses, out)

		out, err = cfg.Get("bootstrap")
		assert.NoError(t, err)
		assert.Equal(t, cfg.Bootstrap, out)

		out, err = cfg.Get("datastore.path")
		assert.NoError(t, err)
		assert.Equal(t, cfg.Datastore.Path, out)

		// TODO we can test this as soon as we have bootstrap addresses.
		// out, err = cfg.Get("bootstrap.addresses.0")
		// assert.NoError(err)
		// assert.Equal(cfg.Bootstrap.Addresses[0], out)
	})

	t.Run("invalid gets", func(t *testing.T) {
		cfg := NewDefaultConfig()

		_, err := cfg.Get("datastore.")
		assert.Error(t, err)

		_, err = cfg.Get(".datastore")
		assert.Error(t, err)

		_, err = cfg.Get("invalidfield")
		assert.Error(t, err)

		_, err = cfg.Get("bootstrap.addresses.toomuch")
		assert.Error(t, err)

		_, err = cfg.Get("api-address")
		assert.Error(t, err)

		// TODO: temporary as we don't have any ATM.
		_, err = cfg.Get("bootstrap.addresses.0")
		assert.Error(t, err)
	})
}

func TestConfigSet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("set leaf values", func(t *testing.T) {
		cfg := NewDefaultConfig()

		// set string
		err := cfg.Set("api.address", `"/ip4/127.9.9.9/tcp/0"`)
		assert.NoError(t, err)
		assert.Equal(t, cfg.API.Address, "/ip4/127.9.9.9/tcp/0")

		// set slice
		err = cfg.Set("api.accessControlAllowOrigin", `["http://localroast:7854"]`)
		assert.NoError(t, err)
		assert.Equal(t, cfg.API.AccessControlAllowOrigin, []string{"http://localroast:7854"})
	})

	t.Run("set table value", func(t *testing.T) {
		cfg := NewDefaultConfig()

		jsonBlob := `{"type": "badgerbadgerbadgerds", "path": "mushroom-mushroom"}`
		err := cfg.Set("datastore", jsonBlob)
		assert.NoError(t, err)
		assert.Equal(t, cfg.Datastore.Type, "badgerbadgerbadgerds")
		assert.Equal(t, cfg.Datastore.Path, "mushroom-mushroom")

		cfg1path, cleaner, err := createConfigFile(fmt.Sprintf(`{"datastore": %s}`, jsonBlob))
		assert.NoError(t, err)
		defer func() {
			require.NoError(t, cleaner())
		}()

		cfg1, err := ReadFile(cfg1path)
		assert.NoError(t, err)
		assert.Equal(t, cfg1.Datastore, cfg.Datastore)

		// inline tables
		jsonBlob = `{"type": "badgerbadgerbadgerds", "path": "mushroom-mushroom"}`
		err = cfg.Set("datastore", jsonBlob)
		assert.NoError(t, err)

		assert.Equal(t, cfg1.Datastore, cfg.Datastore)
	})

	t.Run("invalid set", func(t *testing.T) {
		cfg := NewDefaultConfig()

		// bad key
		err := cfg.Set("datastore.nope", `"too bad, fake key"`)
		assert.Error(t, err)

		// not json
		err = cfg.Set("bootstrap.addresses", `nota.json?key`)
		assert.Error(t, err)

		// newlines in inline tables are invalid
		tomlB := `{type = "badgerbadgerbadgerds",
path = "mushroom-mushroom"}`
		err = cfg.Set("datastore", tomlB)
		assert.Error(t, err)

		// setting values of wrong type
		err = cfg.Set("datastore.type", `["not a", "string"]`)
		assert.Error(t, err)

		err = cfg.Set("bootstrap.addresses", `"not a list"`)
		assert.Error(t, err)

		err = cfg.Set("api", `"strings aren't structs"`)
		assert.Error(t, err)

		// Corrupt address won't pass checksum
		//err = cfg.Set("mining.defaultAddress", "fcqv3gmsd9gd7dqfe60d28euf4tx9v7929corrupt")
		//assert.Contains(err.Error(), "invalid")

		err = cfg.Set("wallet.defaultAddress", "corruptandtooshort")
		assert.Contains(t, err.Error(), address.ErrUnknownNetwork.Error())
	})

	t.Run("setting leaves does not interfere with neighboring leaves", func(t *testing.T) {
		cfg := NewDefaultConfig()

		err := cfg.Set("bootstrap.period", `"3m"`)
		assert.NoError(t, err)
		err = cfg.Set("bootstrap.minPeerThreshold", `5`)
		assert.NoError(t, err)
		assert.Equal(t, cfg.Bootstrap.Period, "3m")
	})
}

func createConfigFile(content string) (string, func() error, error) {
	dir, err := ioutil.TempDir("", "config")
	if err != nil {
		return "", nil, err
	}
	cfgpath := filepath.Join(dir, "config.json")

	if err := ioutil.WriteFile(cfgpath, []byte(content), 0644); err != nil {
		return "", nil, err
	}

	return cfgpath, func() error {
		return os.RemoveAll(dir)
	}, nil
}
