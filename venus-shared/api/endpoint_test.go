package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHost(t *testing.T) {

	cases := []struct {
		raw      string
		ver      uint32
		expected string
	}{
		// valid
		{
			raw:      "http://api.venus.io:1234",
			ver:      1,
			expected: "http://api.venus.io:1234/rpc/v1",
		},
		{
			raw:      "http://api.venus.io:1234/",
			ver:      1,
			expected: "http://api.venus.io:1234/rpc/v1",
		},
		{
			raw:      "http://api.venus.io:1234/rpc",
			ver:      1,
			expected: "http://api.venus.io:1234/rpc",
		},

		{
			raw:      "http://api.venus.io:1234/rpc/v0",
			ver:      1,
			expected: "http://api.venus.io:1234/rpc/v0",
		},

		// invalid: no scheme
		{
			raw:      "://api.venus.io:1234",
			ver:      1,
			expected: "",
		},
		{
			raw:      "://api.venus.io:1234/",
			ver:      1,
			expected: "",
		},
		{
			raw:      "://api.venus.io:1234/rpc",
			ver:      1,
			expected: "",
		},
		{
			raw:      "://api.venus.io:1234/rpc/v0",
			ver:      1,
			expected: "",
		},

		// invalid: no scheme 2
		{
			raw:      "api.venus.io:1234",
			ver:      1,
			expected: "",
		},
		{
			raw:      "api.venus.io:1234/",
			ver:      1,
			expected: "",
		},
		{
			raw:      "api.venus.io:1234/rpc",
			ver:      1,
			expected: "",
		},
		{
			raw:      "api.venus.io:1234/rpc/v0",
			ver:      1,
			expected: "",
		},
	}

	for i := range cases {
		c := cases[i]
		got, err := Endpoint(c.raw, c.ver)
		if c.expected == "" {
			require.Errorf(t, err, "%s is expected to be invalid, got %s", c.raw, got)
			continue
		}

		require.NoErrorf(t, err, "%s is expected to be valid", c.raw)
		require.Equal(t, c.expected, got, "converted endpoint")
	}
}
