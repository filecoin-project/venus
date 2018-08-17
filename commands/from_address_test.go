package commands

import (
	"testing"

	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionalAddr(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)

	t.Run("when option is specified", func(t *testing.T) {
		t.Parallel()

		opts := make(cmdkit.OptMap)

		hash := types.AddressHash([]byte("a new test address"))

		specifiedAddr := types.NewMainnetAddress(hash)
		opts["from"] = specifiedAddr.String()

		addr, err := optionalAddr(opts["from"])
		require.NoError(err)
		assert.Equal(specifiedAddr, addr)
	})

	t.Run("when no option specified return empty", func(t *testing.T) {
		t.Parallel()

		opts := make(cmdkit.OptMap)

		addr, err := optionalAddr(opts["from"])
		require.NoError(err)
		assert.Equal(types.Address{}, addr)
	})
}
