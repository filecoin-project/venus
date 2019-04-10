package version_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/util/version"
)

func TestCheck(t *testing.T) {
	tf.UnitTest(t)

	assert := assert.New(t)

	// Filecoin currently requires go >= 1.12.1
	assert.True(version.Check("go1.12.1"))
	assert.True(version.Check("go1.12.2"))
	assert.True(version.Check("go1.13"))
	assert.True(version.Check("go1.13.1"))

	assert.False(version.Check("go1.11"))
	assert.False(version.Check("go1.11.1"))
	assert.False(version.Check("go1.11.2"))
	assert.False(version.Check("go1.10"))
	assert.False(version.Check("go2"))
}
