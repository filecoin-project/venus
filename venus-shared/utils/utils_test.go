package utils

import (
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestNetworkNamtToNetworkType(t *testing.T) {
	tf.UnitTest(t)
	for nt, nn := range TypeName {
		got, err := NetworkNameToNetworkType(nn)
		assert.Nil(t, err)
		assert.Equal(t, nt, got)
	}

	_, err2 := NetworkNameToNetworkType("2k")
	assert.Equal(t, ErrMay2kNetwork, err2)
}
