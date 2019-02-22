package types

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	"github.com/filecoin-project/go-filecoin/crypto"
)

func TestKeyInfoMarshal(t *testing.T) {
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
