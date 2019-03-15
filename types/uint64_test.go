package types

import (
	"encoding/json"
	"testing"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestUint64CBor(t *testing.T) {
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
