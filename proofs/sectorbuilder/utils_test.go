package sectorbuilder

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/address"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestSectorBuilderUtils(t *testing.T) {
	t.Run("prover id creation", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		addr, err := address.NewActorAddress([]byte("satoshi"))
		require.NoError(err)

		id := AddressToProverID(addr)

		require.Equal(31, len(id))
	})
}
