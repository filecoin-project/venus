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

	assert := assert.New(t)
	require := require.New(t)

	t.Run("when option is specified", func(t *testing.T) {

		opts := make(cmdkit.OptMap)

		specifiedAddr, err := address.NewActorAddress([]byte("a new test address"))
		require.NoError(err)
		opts["from"] = specifiedAddr.String()

		addr, err := optionalAddr(opts["from"])
		require.NoError(err)
		assert.Equal(specifiedAddr, addr)
	})

	t.Run("when no option specified return empty", func(t *testing.T) {

		opts := make(cmdkit.OptMap)

		addr, err := optionalAddr(opts["from"])
		require.NoError(err)
		assert.Equal(address.Undef, addr)
	})
}
