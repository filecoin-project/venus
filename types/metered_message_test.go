package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestMeteredMessageMessage(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()

	t.Run("marshal and equality", func(t *testing.T) {
		inner := NewMessage(
			addrGetter(),
			addrGetter(),
			42,
			NewAttoFILFromFIL(17777),
			"send",
			[]byte("foobar"),
		)

		mmsg := NewMeteredMessage(*inner, NewAttoFILFromFIL(2), NewGasUnits(300))

		// This check requests that you add a non-zero value for new fields above,
		// then update the field count below.
		require.Equal(t, 3, reflect.TypeOf(*mmsg).NumField())

		marshalled, err := mmsg.Marshal()
		assert.NoError(t, err)

		msgBack := MeteredMessage{}
		assert.False(t, mmsg.Equals(&msgBack))

		err = msgBack.Unmarshal(marshalled)
		assert.NoError(t, err)

		assert.Equal(t, mmsg.To, msgBack.To)
		assert.Equal(t, mmsg.From, msgBack.From)
		assert.Equal(t, mmsg.Value, msgBack.Value)
		assert.Equal(t, mmsg.Method, msgBack.Method)
		assert.Equal(t, mmsg.Params, msgBack.Params)
		assert.Equal(t, mmsg.GasPrice, msgBack.GasPrice)
		assert.Equal(t, mmsg.GasLimit, msgBack.GasLimit)

		assert.True(t, mmsg.Equals(&msgBack))
	})
}
