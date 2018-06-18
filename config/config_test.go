package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaults(t *testing.T) {
	assert := assert.New(t)

	cfg := NewDefaultConfig()

	bs := []string{}
	assert.Equal(":3453", cfg.API.Address)
	assert.Equal("/ip4/127.0.0.1/tcp/6000", cfg.Swarm.Address)
	assert.Equal(bs, cfg.Bootstrap.Addresses)
}

func TestWriteFile(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "config")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := NewDefaultConfig()

	assert.NoError(cfg.WriteFile(filepath.Join(dir, "config.toml")))
	content, err := ioutil.ReadFile(filepath.Join(dir, "config.toml"))
	assert.NoError(err)

	assert.Equal(
		`[api]
  address = ":3453"
  accessControlAllowOrigin = ["http://localhost", "https://localhost", "http://127.0.0.1", "https://127.0.0.1"]
  accessControlAllowCredentials = false
  accessControlAllowMethods = ["GET", "POST", "PUT"]

[bootstrap]
  addresses = []

[datastore]
  type = "badgerds"
  path = "badger"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"

[mining]
  rewardAddress = ""
  minerAddresses = []

[wallet]
  defaultAddress = ""
`,
		string(content),
	)

	assert.NoError(os.Remove(filepath.Join(dir, "config.toml")))
}

func TestConfigRoundtrip(t *testing.T) {
	assert := assert.New(t)

	dir, err := ioutil.TempDir("", "config")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	cfg := NewDefaultConfig()

	cfgpath := filepath.Join(dir, "config.toml")
	assert.NoError(cfg.WriteFile(cfgpath))

	cfgout, err := ReadFile(cfgpath)
	assert.NoError(err)

	assert.Equal(cfg, cfgout)
}

func TestConfigReadFileDefaults(t *testing.T) {
	t.Run("all sections exist", func(t *testing.T) {
		assert := assert.New(t)

		cfgpath, cleaner, err := createConfigFile(`
[api]
address = ":9999"
# ignored
other = false

[swarm]
# ignored
other = "hello"
`)
		assert.NoError(err)
		defer cleaner()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(err)

		assert.Equal(cfg.API.Address, ":9999")
		assert.Equal(cfg.Swarm.Address, "/ip4/127.0.0.1/tcp/6000")
	})

	t.Run("missing one section", func(t *testing.T) {
		assert := assert.New(t)

		cfgpath, cleaner, err := createConfigFile(`
[api]
address = ":9999"
# ignored
other = false
`)
		assert.NoError(err)
		defer cleaner()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(err)

		assert.Equal(cfg.API.Address, ":9999")
		assert.Equal(cfg.Swarm.Address, "/ip4/127.0.0.1/tcp/6000")
	})

	t.Run("empty file", func(t *testing.T) {
		assert := assert.New(t)

		cfgpath, cleaner, err := createConfigFile("")
		assert.NoError(err)
		defer cleaner()
		cfg, err := ReadFile(cfgpath)
		assert.NoError(err)

		assert.Equal(cfg.API.Address, ":3453")
		assert.Equal(cfg.Swarm.Address, "/ip4/127.0.0.1/tcp/6000")
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
		out, err := cfg.Set("api.address", `"/ip4/127.9.9.9/tcp/0"`)
		assert.NoError(err)
		assert.Equal(cfg.API.Address, "/ip4/127.9.9.9/tcp/0")
		assert.Equal(out, cfg.API.Address)

		// set slice
		out, err = cfg.Set("api.accessControlAllowOrigin",
			`["http://localroast:7854"]`)
		assert.NoError(err)
		assert.Equal(cfg.API.AccessControlAllowOrigin,
			[]string{"http://localroast:7854"})
		assert.Equal(out, cfg.API.AccessControlAllowOrigin)

		// set slice element
		out, err = cfg.Set("api.accessControlAllowOrigin.0", `"http://localhost:1234"`)
		assert.NoError(err)
		assert.Equal(cfg.API.AccessControlAllowOrigin[0], "http://localhost:1234")
		assert.Equal(out, cfg.API.AccessControlAllowOrigin[0])
	})

	t.Run("set table value", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		tomlBlob := `type = "badgerbadgerbadgerds"
path = "mushroom-mushroom"`
		out, err := cfg.Set("datastore", tomlBlob)
		assert.NoError(err)
		assert.Equal(cfg.Datastore.Type, "badgerbadgerbadgerds")
		assert.Equal(cfg.Datastore.Path, "mushroom-mushroom")
		assert.Equal(out, cfg.Datastore)

		cfg1path, cleaner, err := createConfigFile(
			"[datastore]\n" + tomlBlob)
		assert.NoError(err)
		defer cleaner()

		cfg1, err := ReadFile(cfg1path)
		assert.NoError(err)
		assert.Equal(cfg1.Datastore, cfg.Datastore)

		// inline tables
		tomlBlob = `  {type = "badgerbadgerbadgerds", path = "mushroom-mushroom"}`
		out, err = cfg.Set("datastore", tomlBlob)
		assert.NoError(err)

		assert.Equal(out, cfg.Datastore)
		assert.Equal(cfg1.Datastore, cfg.Datastore)
	})

	t.Run("invalid set", func(t *testing.T) {
		assert := assert.New(t)
		cfg := NewDefaultConfig()

		// bad key
		_, err := cfg.Set("datastore.nope", `"too bad, fake key"`)
		assert.Error(err)

		// index out of bounds
		_, err = cfg.Set("bootstrap.addresses.45", `"555"`)
		assert.Error(err)
		_, err = cfg.Set("bootstrap.addresses.-1", `"555"`)
		assert.Error(err)

		// not TOML
		_, err = cfg.Set("bootstrap.addresses", `nota.toml?key`)
		assert.Error(err)

		// newlines in inline tables are invalid
		tomlB := `{type = "badgerbadgerbadgerds",                      
path = "mushroom-mushroom"}`
		_, err = cfg.Set("datastore", tomlB)
		assert.Error(err)

		// setting values of wrong type
		_, err = cfg.Set("datastore.type", `["not a", "string"]`)
		assert.Error(err)

		_, err = cfg.Set("bootstrap.addresses", `"not a list"`)
		assert.Error(err)

		_, err = cfg.Set("api", `"strings aren't structs"`)
		assert.Error(err)

		// Corrupt address won't pass checksum
		_, err = cfg.Set("mining.rewardAddress",
			`"fcqv3gmsd9gd7dqfe60d28euf4tx9v7929corrupt"`)
		assert.Error(err)

		_, err = cfg.Set("mining.rewardAddress", `"corruptandtooshort"`)
		assert.Error(err)
	})
}

func createConfigFile(content string) (string, func(), error) {
	dir, err := ioutil.TempDir("", "config")
	if err != nil {
		return "", nil, err
	}
	cfgpath := filepath.Join(dir, "config.toml")

	if err := ioutil.WriteFile(cfgpath, []byte(content), 0644); err != nil {
		return "", nil, err
	}

	return cfgpath, func() { os.RemoveAll(dir) }, nil
}
