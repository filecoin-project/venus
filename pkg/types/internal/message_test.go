package internal

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestMessageMarshal(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := NewForTestGetter()
	msg := NewMeteredMessage(
		addrGetter(),
		addrGetter(),
		42,
		NewAttoFILFromFIL(17777),
		0,
		[]byte("foobar"),
		NewAttoFILFromFIL(3),
		NewAttoFILFromFIL(3),
		4,
	)

	// This check requests that you add a non-zero value for new fields above,
	// then update the field count below.
	require.Equal(t, 10, reflect.TypeOf(*msg).NumField())

	buf := new(bytes.Buffer)
	err := msg.MarshalCBOR(buf)
	assert.NoError(t, err)

	msgBack := UnsignedMessage{}
	assert.False(t, msg.Equals(&msgBack))

	err = msgBack.UnmarshalCBOR(buf)
	assert.NoError(t, err)

	assert.Equal(t, msg.Version, msgBack.Version)
	assert.Equal(t, msg.To, msgBack.To)
	assert.Equal(t, msg.From, msgBack.From)
	assert.Equal(t, msg.Value, msgBack.Value)
	assert.Equal(t, msg.Method, msgBack.Method)
	assert.Equal(t, msg.Params, msgBack.Params)
	assert.Equal(t, msg.GasLimit, msgBack.GasLimit)
	assert.Equal(t, msg.GasFeeCap, msgBack.GasFeeCap)
	assert.Equal(t, msg.GasPremium, msgBack.GasPremium)
	assert.True(t, msg.Equals(&msgBack))
}

func TestMessageCid(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := NewForTestGetter()

	msg1 := NewUnsignedMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(999),
		0,
		nil,
	)

	msg2 := NewUnsignedMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(4004),
		0,
		nil,
	)

	c1 := msg1.Cid()
	c2 := msg2.Cid()

	assert.NotEqual(t, c1.String(), c2.String())
}

func TestMessageString(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := NewForTestGetter()

	msg := NewUnsignedMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(999),
		0,
		nil,
	)

	cid := msg.Cid()
	got := msg.String()
	assert.Contains(t, got, cid.String())
}
