package version_test

import (
	"github.com/filecoin-project/go-filecoin/util/version"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestCheck(t *testing.T) {
	tf.UnitTest(t)

	// Filecoin currently requires go >= 1.12.1
	assert.True(t, version.Check("go1.12.1"))
	assert.True(t, version.Check("go1.12.2"))
	assert.True(t, version.Check("go1.13"))
	assert.True(t, version.Check("go1.13.1"))

	assert.False(t, version.Check("go1.11"))
	assert.False(t, version.Check("go1.11.1"))
	assert.False(t, version.Check("go1.11.2"))
	assert.False(t, version.Check("go1.10"))
	assert.False(t, version.Check("go2"))
}
