package account

import (
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestAccountActorCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("CBOR decode(encode(Actor)) == identity(Actor)", func(t *testing.T) {
		preEncode, _ := NewActor(types.NewAttoFILFromFIL(100))
		out, err := cbor.DumpObject(preEncode)
		require.NoError(t, err)

		var postDecode actor.Actor
		err = cbor.DecodeInto(out, &postDecode)
		require.NoError(t, err)

		c1, _ := preEncode.Cid()
		require.NoError(t, err)

		c2, _ := postDecode.Cid()
		require.NoError(t, err)

		types.AssertCidsEqual(t, c1, c2)
	})
}
