package version_test

import (
	"github.com/filecoin-project/go-filecoin/util/version"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestCheck(t *testing.T) {
	assert := assert.New(t)

	assert.True(version.Check("go1.11.1"))
	assert.True(version.Check("go1.11.2"))

	// filecoin currently doesn't work with go 1.12+
	assert.False(version.Check("go1.12"))
	assert.False(version.Check("go1.12.1"))
	assert.False(version.Check("go1.12.2"))

	assert.False(version.Check("go1.11"))
	assert.False(version.Check("go1.10"))
	assert.False(version.Check("go2"))
}
