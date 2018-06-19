package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageMarshal(t *testing.T) {
	assert := assert.New(t)
	addrGetter := NewAddressForTestGetter()

	// TODO: allow more types than just strings for the params
	// currently []interface{} results in type information getting lost when doing
	// a roundtrip with the default cbor encoder.
	msg := NewMessage(
		addrGetter(),
		addrGetter(),
		0,
		NewAttoFILFromFIL(17777),
		"send",
		[]byte("foobar"),
	)

	marshalled, err := msg.Marshal()
	assert.NoError(err)

	msgBack := Message{}
	err = msgBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(msg.To, msgBack.To)
	assert.Equal(msg.From, msgBack.From)
	assert.Equal(msg.Value, msgBack.Value)
	assert.Equal(msg.Method, msgBack.Method)
	assert.Equal(msg.Params, msgBack.Params)
}

func TestMessageCid(t *testing.T) {
	assert := assert.New(t)
	addrGetter := NewAddressForTestGetter()

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
	assert.NoError(err)
	c2, err := msg2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}
