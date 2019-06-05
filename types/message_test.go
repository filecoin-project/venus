package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestMessageMarshal(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()

	// TODO: allow more types than just strings for the params
	// currently []interface{} results in type information getting lost when doing
	// a roundtrip with the default cbor encoder.
	msg := NewMessage(
		addrGetter(),
		addrGetter(),
		42,
		NewAttoFILFromFIL(17777),
		"send",
		[]byte("foobar"),
	)

	// This check requests that you add a non-zero value for new fields above,
	// then update the field count below.
	require.Equal(t, 6, reflect.TypeOf(*msg).NumField())

	marshalled, err := msg.Marshal()
	assert.NoError(t, err)

	msgBack := Message{}
	assert.False(t, msg.Equals(&msgBack))

	err = msgBack.Unmarshal(marshalled)
	assert.NoError(t, err)

	assert.Equal(t, msg.To, msgBack.To)
	assert.Equal(t, msg.From, msgBack.From)
	assert.Equal(t, msg.Value, msgBack.Value)
	assert.Equal(t, msg.Method, msgBack.Method)
	assert.Equal(t, msg.Params, msgBack.Params)
	assert.True(t, msg.Equals(&msgBack))
}

func TestMessageCid(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()

	msg1 := NewMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(999),
		"send",
		nil,
	)

	msg2 := NewMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(4004),
		"send",
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

	addrGetter := address.NewForTestGetter()

	msg := NewMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(999),
		"send",
		nil,
	)

	cid, err := msg.Cid()
	require.NoError(t, err)

	got := msg.String()
	assert.Contains(t, got, cid.String())
}
