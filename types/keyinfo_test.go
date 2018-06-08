package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyInfoMarshal(t *testing.T) {
	assert := assert.New(t)

	testKey := []byte("privat key here")
	testType := "test_key_type"
	ki := &KeyInfo{
		PrivateKey: testKey,
		Curve:      testType,
	}

	marshaled, err := ki.Marshal()
	assert.NoError(err)

	kiBack := &KeyInfo{}
	err = kiBack.Unmarshal(marshaled)
	assert.NoError(err)

	assert.Equal(ki.Key(), kiBack.Key())
	assert.Equal(ki.Type(), kiBack.Type())
	assert.True(ki.Equals(kiBack))
}
