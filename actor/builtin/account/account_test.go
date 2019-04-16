package account

import (
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountActorCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

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
