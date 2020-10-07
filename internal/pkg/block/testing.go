package block

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(t *testing.T, blks ...*Block) *TipSet {
	ts, err := NewTipSet(blks...)
	require.NoError(t, err)
	return ts
}
