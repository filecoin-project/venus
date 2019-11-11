package version

import (
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"
)

const network = "testnetwork"

func TestUpgradeTable(t *testing.T) {
	tf.UnitTest(t)

	t.Run("add single upgrade", func(t *testing.T) {
		version := uint64(3)
		put, err := NewProtocolVersionTableBuilder(network).
			Add(network, version, types.NewBlockHeight(0)).
			Build()
		require.NoError(t, err)

		versionAtHeight, err := put.VersionAt(types.NewBlockHeight(0))
		require.NoError(t, err)

		assert.Equal(t, version, versionAtHeight)

		versionAtHeight, err = put.VersionAt(types.NewBlockHeight(1000))
		require.NoError(t, err)

		assert.Equal(t, version, versionAtHeight)
	})

	t.Run("finds correct version", func(t *testing.T) {
		// add out of order and expect table to sort
		put, err := NewProtocolVersionTableBuilder(network).
			Add(network, 2, types.NewBlockHeight(20)).
			Add(network, 4, types.NewBlockHeight(40)).
			Add(network, 3, types.NewBlockHeight(30)).
			Add(network, 1, types.NewBlockHeight(10)).
			Add(network, 0, types.NewBlockHeight(0)).
			Build()
		require.NoError(t, err)

		for i := uint64(0); i < 50; i++ {
			version, err := put.VersionAt(types.NewBlockHeight(i))
			require.NoError(t, err)

			assert.Equal(t, i/10, version)
		}
	})

	t.Run("constructing a table with no versions is an error", func(t *testing.T) {
		_, err := NewProtocolVersionTableBuilder(network).Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no protocol versions specified for network testnetwork")
	})

	t.Run("constructing a table with no version at genesis is an error", func(t *testing.T) {
		_, err := NewProtocolVersionTableBuilder(network).
			Add(network, 2, types.NewBlockHeight(20)).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no protocol version at genesis for network testnetwork")
	})

	t.Run("ignores versions from wrong network", func(t *testing.T) {
		otherNetwork := "othernetwork"

		put, err := NewProtocolVersionTableBuilder(network).
			Add(network, 0, types.NewBlockHeight(0)).
			Add(otherNetwork, 1, types.NewBlockHeight(10)).
			Add(otherNetwork, 2, types.NewBlockHeight(20)).
			Add(network, 3, types.NewBlockHeight(30)).
			Add(otherNetwork, 4, types.NewBlockHeight(40)).
			Build()
		require.NoError(t, err)

		for i := uint64(0); i < 50; i++ {
			version, err := put.VersionAt(types.NewBlockHeight(i))
			require.NoError(t, err)

			expectedVersion := uint64(0)
			if i >= 30 {
				expectedVersion = 3
			}
			assert.Equal(t, expectedVersion, version)
		}
	})

	t.Run("does not permit the same version number twice", func(t *testing.T) {
		_, err := NewProtocolVersionTableBuilder(network).
			Add(network, 0, types.NewBlockHeight(0)).
			Add(network, 1, types.NewBlockHeight(10)).
			Add(network, 2, types.NewBlockHeight(20)).
			Add(network, 2, types.NewBlockHeight(30)). // wrong
			Add(network, 4, types.NewBlockHeight(40)).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "protocol version 2 effective at 30 is not greater than previous version, 2")
	})

	t.Run("does not permit version numbers to decline", func(t *testing.T) {
		_, err := NewProtocolVersionTableBuilder(network).
			Add(network, 4, types.NewBlockHeight(0)).
			Add(network, 3, types.NewBlockHeight(10)).
			Add(network, 2, types.NewBlockHeight(20)).
			Add(network, 1, types.NewBlockHeight(30)).
			Add(network, 0, types.NewBlockHeight(40)).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "protocol version 3 effective at 10 is not greater than previous version, 4")
	})
}
