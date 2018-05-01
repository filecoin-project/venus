package account

import (
	"testing"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountActorCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(Actor)) == identity(Actor)", func(t *testing.T) {
		require := require.New(t)

		preEncode, _ := NewActor(types.NewTokenAmount(100))
		out, err := cbor.DumpObject(preEncode)
		require.NoError(err)

		var postDecode types.Actor
		err = cbor.DecodeInto(out, &postDecode)
		require.NoError(err)

		c1, _ := preEncode.Cid()
		require.NoError(err)

		c2, _ := postDecode.Cid()
		require.NoError(err)

		types.AssertCidsEqual(assert.New(t), c1, c2)
	})
}
