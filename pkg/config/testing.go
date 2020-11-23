package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Makes some basic checks of a serialized config to ascertain that it looks kind of right.
// This is instead of brittle hardcoded exact config expectations.
func SanityCheck(t *testing.T, cfgJSON string) {
	assert.True(t, strings.Contains(cfgJSON, "accessControlAllowOrigin"))
	assert.True(t, strings.Contains(cfgJSON, "http://localhost:8080"))
	assert.True(t, strings.Contains(cfgJSON, "bootstrap"))
	assert.True(t, strings.Contains(cfgJSON, "bootstrap"))
	assert.True(t, strings.Contains(cfgJSON, "\"minPeerThreshold\": 3"))
}
