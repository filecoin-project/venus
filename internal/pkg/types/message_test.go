package types

import (
	"reflect"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

func TestMessageMarshal(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := vmaddr.NewForTestGetter()

	// TODO: allow more types than just strings for the params
	// currently []interface{} results in type information getting lost when doing
	// a roundtrip with the default cbor encoder.
	msg := NewMeteredMessage(
		addrGetter(),
		addrGetter(),
		42,
		NewAttoFILFromFIL(17777),
		builtin.MethodSend,
		[]byte("foobar"),
		NewAttoFILFromFIL(3),
		GasUnits(4),
	)

	// This check requests that you add a non-zero value for new fields above,
	// then update the field count below.
	require.Equal(t, 9, reflect.TypeOf(*msg).NumField())

	marshalled, err := msg.Marshal()
	assert.NoError(t, err)

	msgBack := UnsignedMessage{}
	assert.False(t, msg.Equals(&msgBack))

	err = msgBack.Unmarshal(marshalled)
	assert.NoError(t, err)

	assert.Equal(t, msg.To, msgBack.To)
	assert.Equal(t, msg.From, msgBack.From)
	assert.Equal(t, msg.Value, msgBack.Value)
	assert.Equal(t, msg.Method, msgBack.Method)
	assert.Equal(t, msg.Params, msgBack.Params)
	assert.Equal(t, msg.GasLimit, msgBack.GasLimit)
	assert.Equal(t, msg.GasPrice, msgBack.GasPrice)
	assert.True(t, msg.Equals(&msgBack))
}

func TestMessageCid(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := vmaddr.NewForTestGetter()

	msg1 := NewUnsignedMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(999),
		builtin.MethodSend,
		nil,
	)

	msg2 := NewUnsignedMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(4004),
		builtin.MethodSend,
		nil,
	)

	c1, err := msg1.Cid()
	assert.NoError(t, err)
	c2, err := msg2.Cid()
	assert.NoError(t, err)

	assert.NotEqual(t, c1.String(), c2.String())
}

func TestMessageString(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := vmaddr.NewForTestGetter()

	msg := NewUnsignedMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(999),
		builtin.MethodSend,
		nil,
	)

	cid, err := msg.Cid()
	require.NoError(t, err)

	got := msg.String()
	assert.Contains(t, got, cid.String())
}
