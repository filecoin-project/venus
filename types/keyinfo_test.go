package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/crypto"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestKeyInfoMarshal(t *testing.T) {
	tf.UnitTest(t)

	testKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	testType := "test_key_type"
	ki := &KeyInfo{
		PrivateKey: testKey,
		Curve:      testType,
	}

	marshaled, err := ki.Marshal()
	assert.NoError(t, err)

	kiBack := &KeyInfo{}
	err = kiBack.Unmarshal(marshaled)
	assert.NoError(t, err)

	assert.Equal(t, ki.Key(), kiBack.Key())
	assert.Equal(t, ki.Type(), kiBack.Type())
	assert.True(t, ki.Equals(kiBack))
}
