package types

import (
	"encoding/json"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestUint64CBor(t *testing.T) {
	tf.UnitTest(t)

	v := Uint64(64)
	m, err := cbor.DumpObject(v)
	assert.NoError(t, err)
	var got Uint64
	err = cbor.DecodeInto(m, &got)
	assert.NoError(t, err)
	assert.Equal(t, v, got)
}

func TestUint64Json(t *testing.T) {
	tf.UnitTest(t)

	v := Uint64(64)
	m, err := json.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, `"64"`, string(m))
	var got Uint64
	err = json.Unmarshal(m, &got)
	assert.NoError(t, err)
	assert.Equal(t, v, got)
}
