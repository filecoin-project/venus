package version

import (
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"

	"testing"
)

const network = "testnetwork"

func TestUpgradeTable(t *testing.T) {
	tf.UnitTest(t)

	t.Run("add single upgrade", func(t *testing.T) {
		version := uint64(3)
		put, err := NewProtocolVersionTableBuilder(network).
			Add(network, version, abi.ChainEpoch(0)).
			Build()
		require.NoError(t, err)

		versionAtHeight, err := put.VersionAt(abi.ChainEpoch(0))
		require.NoError(t, err)

		assert.Equal(t, version, versionAtHeight)

		versionAtHeight, err = put.VersionAt(abi.ChainEpoch(1000))
		require.NoError(t, err)

		assert.Equal(t, version, versionAtHeight)
	})

	t.Run("finds correct version", func(t *testing.T) {
		// add out of order and expect table to sort
		put, err := NewProtocolVersionTableBuilder(network).
			Add(network, 2, abi.ChainEpoch(20)).
			Add(network, 4, abi.ChainEpoch(40)).
			Add(network, 3, abi.ChainEpoch(30)).
			Add(network, 1, abi.ChainEpoch(10)).
			Add(network, 0, abi.ChainEpoch(0)).
			Build()
		require.NoError(t, err)

		for i := uint64(0); i < 50; i++ {
			version, err := put.VersionAt(abi.ChainEpoch(i))
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
			Add(network, 2, abi.ChainEpoch(20)).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no protocol version at genesis for network testnetwork")
	})

	t.Run("ignores versions from wrong network", func(t *testing.T) {
		otherNetwork := "othernetwork"

		put, err := NewProtocolVersionTableBuilder(network).
			Add(network, 0, abi.ChainEpoch(0)).
			Add(otherNetwork, 1, abi.ChainEpoch(10)).
			Add(otherNetwork, 2, abi.ChainEpoch(20)).
			Add(network, 3, abi.ChainEpoch(30)).
			Add(otherNetwork, 4, abi.ChainEpoch(40)).
			Build()
		require.NoError(t, err)

		for i := uint64(0); i < 50; i++ {
			version, err := put.VersionAt(abi.ChainEpoch(i))
			require.NoError(t, err)

			expectedVersion := uint64(0)
			if i >= 30 {
				expectedVersion = 3
			}
			assert.Equal(t, expectedVersion, version)
		}
	})

	t.Run("version table name can be a prefix of network name", func(t *testing.T) {
		network := "localnet-270a8688-1b23-4508-b675-444cb1e6f05d"
		versionName := "localnet"

		put, err := NewProtocolVersionTableBuilder(network).
			Add(versionName, 0, abi.ChainEpoch(0)).
			Add(versionName, 1, abi.ChainEpoch(10)).
			Build()
		require.NoError(t, err)

		for i := uint64(0); i < 20; i++ {
			version, err := put.VersionAt(abi.ChainEpoch(i))
			require.NoError(t, err)

			expectedVersion := uint64(0)
			if i >= 10 {
				expectedVersion = 1
			}
			assert.Equal(t, expectedVersion, version)
		}
	})

	t.Run("does not permit the same version number twice", func(t *testing.T) {
		_, err := NewProtocolVersionTableBuilder(network).
			Add(network, 0, abi.ChainEpoch(0)).
			Add(network, 1, abi.ChainEpoch(10)).
			Add(network, 2, abi.ChainEpoch(20)).
			Add(network, 2, abi.ChainEpoch(30)). // wrong
			Add(network, 4, abi.ChainEpoch(40)).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "protocol version 2 effective at 30 is not greater than previous version, 2")
	})

	t.Run("does not permit version numbers to decline", func(t *testing.T) {
		_, err := NewProtocolVersionTableBuilder(network).
			Add(network, 4, abi.ChainEpoch(0)).
			Add(network, 3, abi.ChainEpoch(10)).
			Add(network, 2, abi.ChainEpoch(20)).
			Add(network, 1, abi.ChainEpoch(30)).
			Add(network, 0, abi.ChainEpoch(40)).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "protocol version 3 effective at 10 is not greater than previous version, 4")
	})
}
