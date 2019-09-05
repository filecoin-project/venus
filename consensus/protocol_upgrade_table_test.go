package consensus

import (
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"testing"
)

func TestUpgradeTable(t *testing.T) {
	tf.UnitTest(t)

	t.Run("add single upgrade", func(t *testing.T) {
		network := "testnetwork"
		version := uint64(3)
		put := NewProtocolUpgradeTable(network)

		put.Add(network, version, types.NewBlockHeight(0))

		versionAtHeight, err := put.VersionAt(types.NewBlockHeight(0))
		require.NoError(t, err)

		assert.Equal(t, version, versionAtHeight)

		versionAtHeight, err = put.VersionAt(types.NewBlockHeight(1000))
		require.NoError(t, err)

		assert.Equal(t, version, versionAtHeight)
	})

	t.Run("finds correct version", func(t *testing.T) {
		network := "testnetwork"
		put := NewProtocolUpgradeTable(network)

		// add out of order and expect table to sort
		put.Add(network, 2, types.NewBlockHeight(20))
		put.Add(network, 4, types.NewBlockHeight(40))
		put.Add(network, 3, types.NewBlockHeight(30))
		put.Add(network, 1, types.NewBlockHeight(10))
		put.Add(network, 0, types.NewBlockHeight(0))

		for i := uint64(0); i < 50; i++ {
			version, err := put.VersionAt(types.NewBlockHeight(i))
			require.NoError(t, err)

			assert.Equal(t, i/10, version)
		}
	})

	t.Run("retrieving from empty table is an error", func(t *testing.T) {
		network := "testnetwork"
		put := NewProtocolUpgradeTable(network)

		_, err := put.VersionAt(types.NewBlockHeight(0))
		require.Error(t, err)
		assert.Matches(t, err.Error(), "no protocol versions for testnetwork network")
	})

	t.Run("retrieving before first upgrade is an error", func(t *testing.T) {
		network := "testnetwork"
		put := NewProtocolUpgradeTable(network)
		put.Add(network, 2, types.NewBlockHeight(20))

		_, err := put.VersionAt(types.NewBlockHeight(0))
		require.Error(t, err)
		assert.Matches(t, err.Error(), "less than effective start")
	})

	t.Run("ignores upgrades from wrong network", func(t *testing.T) {
		network := "testnetwork"
		otherNetwork := "othernetwork"
		put := NewProtocolUpgradeTable(network)

		// add out of order and expect table to sort
		put.Add(network, 0, types.NewBlockHeight(0))
		put.Add(otherNetwork, 1, types.NewBlockHeight(10))
		put.Add(otherNetwork, 2, types.NewBlockHeight(20))
		put.Add(network, 3, types.NewBlockHeight(30))
		put.Add(otherNetwork, 4, types.NewBlockHeight(40))

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

}
