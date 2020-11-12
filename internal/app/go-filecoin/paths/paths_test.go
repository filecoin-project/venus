package paths

import (
	"testing"

	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/require"
)

func TestRepoPathGet(t *testing.T) {
	tf.UnitTest(t)

	t.Run("get default repo path", func(t *testing.T) {
		_, err := GetRepoPath("")

		require.NoError(t, err)
	})
}
