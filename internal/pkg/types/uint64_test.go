package types

import (
	"encoding/json"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestUint64CBor(t *testing.T) {
	tf.UnitTest(t)

	v := Uint64(64)
	m, err := encoding.Encode(v)
	assert.NoError(t, err)
	var got Uint64
	err = encoding.Decode(m, &got)
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
