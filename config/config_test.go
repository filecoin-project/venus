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

	assert.Equal(cfg.API.Address, ":3453")
	assert.Equal(cfg.Swarm.Address, "/ip4/127.0.0.1/tcp/6000")
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
		string(content),
		`[api]
  address = ":3453"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"
`,
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
