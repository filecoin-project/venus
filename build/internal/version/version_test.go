// +build !go1.14

package version_test

import (
	"testing"

	"github.com/filecoin-project/venus/build/internal/version"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestCheck(t *testing.T) {
	tf.UnitTest(t)

	// Filecoin currently requires go >= 1.14
	assert.True(t, version.Check("go1.14"))

	assert.True(t, version.Check("go1.13.1"))
	assert.True(t, version.Check("go1.13.2"))

	assert.False(t, version.Check("go1.12.1"))
	assert.False(t, version.Check("go1.12.2"))

	assert.False(t, version.Check("go1.11"))
	assert.False(t, version.Check("go1.11.1"))
	assert.False(t, version.Check("go1.11.2"))
	assert.False(t, version.Check("go1.10"))
	assert.False(t, version.Check("go2"))
}
