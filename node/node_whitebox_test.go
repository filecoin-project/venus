package node

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestMakePrivateKey(t *testing.T) {
	tf.UnitTest(t)

	// should fail if less than 1024
	badKey, err := makePrivateKey(10)
	assert.Error(t, err, ErrLittleBits)
	assert.Nil(t, badKey)

	// 1024 should work
	okKey, err := makePrivateKey(1024)
	assert.NoError(t, err)
	assert.NotNil(t, okKey)

	// large values should work
	goodKey, err := makePrivateKey(4096)
	assert.NoError(t, err)
	assert.NotNil(t, goodKey)
}
