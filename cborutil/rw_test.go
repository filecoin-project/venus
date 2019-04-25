package cborutil

import (
	"bytes"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
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
	tf.UnitTest(t)

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
		assert.NoError(t, w.WriteMsg(it))
	}

	for _, it := range items {
		var msg fooTestMessage
		assert.NoError(t, r.ReadMsg(&msg))
		assert.Equal(t, it, msg)
	}
}
