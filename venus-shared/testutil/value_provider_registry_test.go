package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidProviders(t *testing.T) {
	vals := []interface{}{
		int(0),
		float32(0),
		func() {},
		func(t *testing.T) {},
		func() int { return 1 },
		func(int) int { return 1 },
	}

	for ri := range vals {
		err := defaultValueProviderRegistry.register(vals[ri])
		require.Errorf(t, err, "value #%d", ri)
	}
}
