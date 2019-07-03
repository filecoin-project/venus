package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestOptionalAddr(t *testing.T) {
	tf.UnitTest(t)

	t.Run("when option is specified", func(t *testing.T) {

		opts := make(cmdkit.OptMap)

		specifiedAddr, err := address.NewActorAddress([]byte("a new test address"))
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
