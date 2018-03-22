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

	cfg := NewDefaultConfig()

	assert.NoError(cfg.WriteFile(filepath.Join(dir, "config.toml")))
	content, err := ioutil.ReadFile(filepath.Join(dir, "config.toml"))
	assert.NoError(err)

	assert.Equal(
		`[api]
  address = ":3453"

[swarm]
  address = "/ip4/127.0.0.1/tcp/6000"

[bootstrap]
  addresses = ["TODO"]
`,
		string(content),
	)

	assert.NoError(os.Remove(filepath.Join(dir, "config.toml")))
}
