package account

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/encoding"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestAccountActorCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("CBOR decode(encode(Actor)) == identity(Actor)", func(t *testing.T) {
		preEncode, _ := NewActor(types.NewAttoFILFromFIL(100))
		out, err := encoding.Encode(preEncode)
		require.NoError(t, err)

		var postDecode actor.Actor
		err = encoding.Decode(out, &postDecode)
		require.NoError(t, err)

		c1, _ := preEncode.Cid()
		require.NoError(t, err)

		c2, _ := postDecode.Cid()
		require.NoError(t, err)

		types.AssertCidsEqual(t, c1, c2)
	})
}
