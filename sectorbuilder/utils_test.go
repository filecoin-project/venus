package sectorbuilder

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/address"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSectorBuilderUtils(t *testing.T) {
	t.Run("prover id creation", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		hash := address.Hash([]byte("satoshi"))
		addr := address.NewMainnet(hash)

		id := addressToProverID(addr)

		require.Equal(31, len(id))
	})

	t.Run("creating datastore keys", func(t *testing.T) {
		t.Parallel()

		assert := assert.New(t)

		label := "SECTORFILENAMEWHATEVER"

		k := metadataKey(label).String()
		// Don't accidentally test Datastore namespacing implementation.
		assert.Contains(k, "sectors")
		assert.Contains(k, "metadata")
		assert.Contains(k, label)

		var merkleRoot [32]byte
		copy(merkleRoot[:], ([]byte)("someMerkleRootLOL")[0:32])

		k2 := sealedMetadataKey(merkleRoot).String()
		// Don't accidentally test Datastore namespacing implementation.
		assert.Contains(k2, "sealedSectors")
		assert.Contains(k2, "metadata")
		assert.Contains(k2, commRString(merkleRoot))
	})
}
