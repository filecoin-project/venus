package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestOptionalAddr(t *testing.T) {
	tf.UnitTest(t)

	t.Run("when option is specified", func(t *testing.T) {

		opts := make(cmdkit.OptMap)

		specifiedAddr, err := address.NewSecp256k1Address([]byte("a new test address"))
		require.NoError(t, err)
		opts["from"] = specifiedAddr.String()

		addr, err := optionalAddr(opts["from"])
		require.NoError(t, err)
		assert.Equal(t, specifiedAddr, addr)
	})

	t.Run("when no option specified return empty", func(t *testing.T) {

		opts := make(cmdkit.OptMap)

		addr, err := optionalAddr(opts["from"])
		require.NoError(t, err)
		assert.Equal(t, address.Undef, addr)
	})
}
