package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
)

func TestDefaults(t *testing.T) {
	assert := assert.New(t)

	cfg := NewDefaultConfig()

	bs := []string{}
	assert.Equal("/ip4/127.0.0.1/tcp/3453", cfg.API.Address)
	assert.Equal("/ip4/0.0.0.0/tcp/6000", cfg.Swarm.Address)
	assert.Equal(bs, cfg.Bootstrap.Addresses)
}

func TestWriteFile(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "config")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := NewDefaultConfig()

	assert.NoError(cfg.WriteFile(filepath.Join(dir, "config.json")))
	content, err := ioutil.ReadFile(filepath.Join(dir, "config.json"))
	assert.NoError(err)

	assert.Equal(
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
	"swarm": {
		"address": "/ip4/0.0.0.0/tcp/6000"
	},
	"mining": {
		"minerAddress": "empty",
		"autoSealIntervalSeconds": 120,
		"storagePrice": "0"
	},
	"wallet": {
		"defaultAddress": "empty"
	},
	"heartbeat": {
		"beatTarget": "",
		"beatPeriod": "3s",
		"reconnectPeriod": "10s",
		"nickname": ""
	},
	"net": ""
}`,
		string(content),
	)

	assert.NoError(os.Remove(filepath.Join(dir, "config.json")))
}

func TestSetRejectsInvalidNicks(t *testing.T) {
	assert := assert.New(t)
	cfg := NewDefaultConfig()

	// sic: json includes the quotes in the value
	err := cfg.Set("heartbeat.nickname", "\"goodnick\"")
	assert.NoError(err)
	err = cfg.Set("heartbeat.nickname", "bad nick<p>")
	assert.Error(err)
	err = cfg.Set("heartbeat", `{"heartbeat": "bad nick"}`)
	assert.Error(err)
}

func TestConfigRoundtrip(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "config")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := NewDefaultConfig()

	cfgpath := filepath.Join(dir, "config.json")
	assert.NoError(cfg.WriteFile(cfgpath))

	cfgout, err := ReadFile(cfgpath)
	assert.NoError(err)

	assert.Equal(cfg, cfgout)
}

func TestConfigReadFileDefaults(t *testing.T) {
	t.Run("all sections exist", func(t *testing.T) {
		assert := assert.New(t)

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
		assert.NoError(err)
		defer cleaner()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(err)

		assert.Equal(cfg.API.Address, "/ip4/127.0.0.1/tcp/9999")
		assert.Equal(cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/6000")
	})

	t.Run("missing one section", func(t *testing.T) {
		assert := assert.New(t)

		cfgpath, cleaner, err := createConfigFile(`
		{
			"api": {
				"address": "/ip4/127.0.0.1/tcp/9999",
				"keyThatDoesntExit'": false
			}
		}`)
		assert.NoError(err)
		defer cleaner()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(err)

		assert.Equal(cfg.API.Address, "/ip4/127.0.0.1/tcp/9999")
		assert.Equal(cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/6000")
	})

	t.Run("empty file", func(t *testing.T) {
		assert := assert.New(t)

		cfgpath, cleaner, err := createConfigFile("")
		assert.NoError(err)
		defer cleaner()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(err)

		assert.Equal(cfg.API.Address, "/ip4/127.0.0.1/tcp/3453")
		assert.Equal(cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/6000")
	})
}

func TestConfigGet(t *testing.T) {
	t.Run("valid gets", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		out, err := cfg.Get("api.address")
		assert.NoError(err)
		assert.Equal(cfg.API.Address, out)

		out, err = cfg.Get("api.accessControlAllowOrigin")
		assert.NoError(err)
		assert.Equal(cfg.API.AccessControlAllowOrigin, out)

		out, err = cfg.Get("api")
		assert.NoError(err)
		assert.Equal(cfg.API, out)

		out, err = cfg.Get("bootstrap.addresses")
		assert.NoError(err)
		assert.Equal(cfg.Bootstrap.Addresses, out)

		out, err = cfg.Get("bootstrap")
		assert.NoError(err)
		assert.Equal(cfg.Bootstrap, out)

		out, err = cfg.Get("datastore.path")
		assert.NoError(err)
		assert.Equal(cfg.Datastore.Path, out)

		// TODO we can test this as soon as we have bootstrap addresses.
		// out, err = cfg.Get("bootstrap.addresses.0")
		// assert.NoError(err)
		// assert.Equal(cfg.Bootstrap.Addresses[0], out)
	})

	t.Run("invalid gets", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		_, err := cfg.Get("datastore.")
		assert.Error(err)

		_, err = cfg.Get(".datastore")
		assert.Error(err)

		_, err = cfg.Get("invalidfield")
		assert.Error(err)

		_, err = cfg.Get("bootstrap.addresses.toomuch")
		assert.Error(err)

		_, err = cfg.Get("api-address")
		assert.Error(err)

		// TODO: temporary as we don't have any ATM.
		_, err = cfg.Get("bootstrap.addresses.0")
		assert.Error(err)
	})
}

func TestConfigSet(t *testing.T) {
	t.Run("set leaf values", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		// set string
		err := cfg.Set("api.address", `"/ip4/127.9.9.9/tcp/0"`)
		assert.NoError(err)
		assert.Equal(cfg.API.Address, "/ip4/127.9.9.9/tcp/0")

		// set slice
		err = cfg.Set("api.accessControlAllowOrigin", `["http://localroast:7854"]`)
		assert.NoError(err)
		assert.Equal(cfg.API.AccessControlAllowOrigin, []string{"http://localroast:7854"})
	})

	t.Run("set table value", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		jsonBlob := `{"type": "badgerbadgerbadgerds", "path": "mushroom-mushroom"}`
		err := cfg.Set("datastore", jsonBlob)
		assert.NoError(err)
		assert.Equal(cfg.Datastore.Type, "badgerbadgerbadgerds")
		assert.Equal(cfg.Datastore.Path, "mushroom-mushroom")

		cfg1path, cleaner, err := createConfigFile(fmt.Sprintf(`{"datastore": %s}`, jsonBlob))
		assert.NoError(err)
		defer cleaner()

		cfg1, err := ReadFile(cfg1path)
		assert.NoError(err)
		assert.Equal(cfg1.Datastore, cfg.Datastore)

		// inline tables
		jsonBlob = `{"type": "badgerbadgerbadgerds", "path": "mushroom-mushroom"}`
		err = cfg.Set("datastore", jsonBlob)
		assert.NoError(err)

		assert.Equal(cfg1.Datastore, cfg.Datastore)
	})

	t.Run("invalid set", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		// bad key
		err := cfg.Set("datastore.nope", `"too bad, fake key"`)
		assert.Error(err)

		// not json
		err = cfg.Set("bootstrap.addresses", `nota.json?key`)
		assert.Error(err)

		// newlines in inline tables are invalid
		tomlB := `{type = "badgerbadgerbadgerds",
path = "mushroom-mushroom"}`
		err = cfg.Set("datastore", tomlB)
		assert.Error(err)

		// setting values of wrong type
		err = cfg.Set("datastore.type", `["not a", "string"]`)
		assert.Error(err)

		err = cfg.Set("bootstrap.addresses", `"not a list"`)
		assert.Error(err)

		err = cfg.Set("api", `"strings aren't structs"`)
		assert.Error(err)

		// Corrupt address won't pass checksum
		//err = cfg.Set("mining.defaultAddress", "fcqv3gmsd9gd7dqfe60d28euf4tx9v7929corrupt")
		//assert.Contains(err.Error(), "invalid")

		err = cfg.Set("wallet.defaultAddress", "corruptandtooshort")
		assert.Contains(err.Error(), address.ErrUnknownNetwork.Error())
	})

	t.Run("setting leaves does not interfere with neighboring leaves", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		err := cfg.Set("bootstrap.period", `"3m"`)
		assert.NoError(err)
		err = cfg.Set("bootstrap.minPeerThreshold", `5`)
		assert.NoError(err)
		assert.Equal(cfg.Bootstrap.Period, "3m")
	})
}

func createConfigFile(content string) (string, func(), error) {
	dir, err := ioutil.TempDir("", "config")
	if err != nil {
		return "", nil, err
	}
	cfgpath := filepath.Join(dir, "config.json")

	if err := ioutil.WriteFile(cfgpath, []byte(content), 0644); err != nil {
		return "", nil, err
	}

	return cfgpath, func() { os.RemoveAll(dir) }, nil
}
