package sectorbuilder

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

func TestSectorBuilderUtils(t *testing.T) {
	tf.UnitTest(t)

	t.Run("prover id creation", func(t *testing.T) {
		addr, err := address.NewSecp256k1Address([]byte("satoshi"))
		require.NoError(t, err)

		id := AddressToProverID(addr)

		require.Equal(t, 31, len(id))
	})
}
