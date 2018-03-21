package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaults(t *testing.T) {
	assert := assert.New(t)

	cfg := NewDefaultConfig()

	assert.Equal(cfg.API.Address, ":3453")
	assert.Equal(cfg.Swarm.Address, "/ip4/127.0.0.1/tcp/6000")
}
