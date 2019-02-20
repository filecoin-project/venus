package account

import (
	"testing"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestAccountActorCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(Actor)) == identity(Actor)", func(t *testing.T) {
		require := require.New(t)

		preEncode, _ := NewActor(types.NewAttoFILFromFIL(100))
		out, err := cbor.DumpObject(preEncode)
		require.NoError(err)

		var postDecode actor.Actor
		err = cbor.DecodeInto(out, &postDecode)
		require.NoError(err)

		c1, _ := preEncode.Cid()
		require.NoError(err)

		c2, _ := postDecode.Cid()
		require.NoError(err)

		types.AssertCidsEqual(assert.New(t), c1, c2)
	})
}
