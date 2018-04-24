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

	bs := []string{
		"TODO",
	}
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
  accessControlAllowOrigin = ["http://localhost:8080"]

[bootstrap]
  addresses = ["TODO"]

[datastore]
  type = "badgerds"
  path = "badger"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"

[mining]
  rewardAddress = ""
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
