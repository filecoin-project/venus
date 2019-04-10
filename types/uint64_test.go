package types

import (
	"encoding/json"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestUint64CBor(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	v := Uint64(64)
	m, err := cbor.DumpObject(v)
	assert.NoError(err)
	var got Uint64
	err = cbor.DecodeInto(m, &got)
	assert.NoError(err)
	assert.Equal(v, got)
}

func TestUint64Json(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	v := Uint64(64)
	m, err := json.Marshal(v)
	assert.NoError(err)
	assert.Equal(`"64"`, string(m))
	var got Uint64
	err = json.Unmarshal(m, &got)
	assert.NoError(err)
	assert.Equal(v, got)
}
