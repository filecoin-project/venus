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

	assert := assert.New(t)
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

		mmsg := NewMeteredMessage(*inner, *NewAttoFILFromFIL(2), NewGasUnits(300))

		// This check requests that you add a non-zero value for new fields above,
		// then update the field count below.
		require.Equal(t, 3, reflect.TypeOf(*mmsg).NumField())

		marshalled, err := mmsg.Marshal()
		assert.NoError(err)

		msgBack := MeteredMessage{}
		assert.False(mmsg.Equals(&msgBack))

		err = msgBack.Unmarshal(marshalled)
		assert.NoError(err)

		assert.Equal(mmsg.To, msgBack.To)
		assert.Equal(mmsg.From, msgBack.From)
		assert.Equal(mmsg.Value, msgBack.Value)
		assert.Equal(mmsg.Method, msgBack.Method)
		assert.Equal(mmsg.Params, msgBack.Params)
		assert.Equal(mmsg.GasPrice, msgBack.GasPrice)
		assert.Equal(mmsg.GasLimit, msgBack.GasLimit)

		assert.True(mmsg.Equals(&msgBack))
	})
}
