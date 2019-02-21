package node

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestMakePrivateKey(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// should fail if less than 1024
	badKey, err := makePrivateKey(10)
	assert.Error(err, ErrLittleBits)
	assert.Nil(badKey)

	// 1024 should work
	okKey, err := makePrivateKey(1024)
	assert.NoError(err)
	assert.NotNil(okKey)

	// large values should work
	goodKey, err := makePrivateKey(4096)
	assert.NoError(err)
	assert.NotNil(goodKey)
}
