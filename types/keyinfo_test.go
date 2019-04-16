package types

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/crypto"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

func TestKeyInfoMarshal(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	testKey, err := crypto.GenerateKey()
	assert.NoError(err)
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
