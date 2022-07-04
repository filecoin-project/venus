package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestDefaults(t *testing.T) {
	tf.UnitTest(t)

	cfg := NewDefaultConfig()

	bs := []string{}
	assert.Equal(t, "/ip4/127.0.0.1/tcp/3453", cfg.API.APIAddress)
	assert.Equal(t, "/ip4/0.0.0.0/tcp/0", cfg.Swarm.Address)
	assert.Equal(t, bs, cfg.Bootstrap.Addresses)
}

func TestWriteFile(t *testing.T) {
	tf.UnitTest(t)

	dir := t.TempDir()

	cfg := NewDefaultConfig()

	cfgJSON, err := json.MarshalIndent(*cfg, "", "\t")
	require.NoError(t, err)
	expected := string(cfgJSON)

	SanityCheck(t, expected)

	assert.NoError(t, cfg.WriteFile(filepath.Join(dir, "config.json")))
	content, err := ioutil.ReadFile(filepath.Join(dir, "config.json"))
	assert.NoError(t, err)

	assert.Equal(t, expected, string(content))
	assert.NoError(t, os.Remove(filepath.Join(dir, "config.json")))
}

func TestConfigRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	dir := t.TempDir()

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
		cfgpath, err := createConfigFile(t, `
		{
			"api": {
				"apiAddress": "/ip4/127.0.0.1/tcp/9999",
				"keyThatDoesntExit": false
			},
			"swarm": {
				"keyThatDoesntExit": "hello"
			}
		}`)
		assert.NoError(t, err)
		cfg, err := ReadFile(cfgpath)
		assert.NoError(t, err)

		assert.Equal(t, cfg.API.APIAddress, "/ip4/127.0.0.1/tcp/9999")
		assert.Equal(t, cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/0")
	})

	t.Run("missing one section", func(t *testing.T) {
		cfgpath, err := createConfigFile(t, `
		{
			"api": {
				"apiAddress": "/ip4/127.0.0.1/tcp/9999",
				"keyThatDoesntExit'": false
			}
		}`)
		assert.NoError(t, err)
		cfg, err := ReadFile(cfgpath)
		assert.NoError(t, err)

		assert.Equal(t, cfg.API.APIAddress, "/ip4/127.0.0.1/tcp/9999")
		assert.Equal(t, cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/0")
	})

	t.Run("empty file", func(t *testing.T) {
		cfgpath, err := createConfigFile(t, "")
		assert.NoError(t, err)
		cfg, err := ReadFile(cfgpath)
		assert.NoError(t, err)

		assert.Equal(t, cfg.API.APIAddress, "/ip4/127.0.0.1/tcp/3453")
		assert.Equal(t, cfg.Swarm.Address, "/ip4/0.0.0.0/tcp/0")
	})
}

func TestConfigGet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("valid gets", func(t *testing.T) {
		cfg := NewDefaultConfig()

		out, err := cfg.Get("api.apiAddress")
		assert.NoError(t, err)
		assert.Equal(t, cfg.API.APIAddress, out)

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
		err := cfg.Set("api.apiAddress", `"/ip4/127.9.9.9/tcp/0"`)
		assert.NoError(t, err)
		assert.Equal(t, cfg.API.APIAddress, "/ip4/127.9.9.9/tcp/0")

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

		cfg1path, err := createConfigFile(t, fmt.Sprintf(`{"datastore": %s}`, jsonBlob))
		assert.NoError(t, err)

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

		err = cfg.Set("walletModule.defaultAddress", "corruptandtooshort")
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

func createConfigFile(t *testing.T, content string) (string, error) {
	cfgpath := filepath.Join(t.TempDir(), "config.json")

	if err := ioutil.WriteFile(cfgpath, []byte(content), 0644); err != nil {
		return "", err
	}

	return cfgpath, nil
}
