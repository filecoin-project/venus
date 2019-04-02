package cborutil

import (
	"bytes"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func init() {
	cbor.RegisterCborType(fooTestMessage{})
}

type fooTestMessage struct {
	A string
	B int
}

func TestMessageSending(t *testing.T) {
	assert := assert.New(t)

	buf := new(bytes.Buffer)
	w := NewMsgWriter(buf)
	r := NewMsgReader(buf)

	items := []fooTestMessage{
		{A: "cat", B: 17},
		{A: "dog", B: 93},
		{A: "bear", B: 41},
		{A: "fish", B: 101},
	}

	for _, it := range items {
		assert.NoError(w.WriteMsg(it))
	}

	for _, it := range items {
		var msg fooTestMessage
		assert.NoError(r.ReadMsg(&msg))
		assert.Equal(it, msg)
	}
}
