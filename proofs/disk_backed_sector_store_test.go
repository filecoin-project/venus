package proofs

import (
	"testing"

	"io/ioutil"

	"os"

	"github.com/stretchr/testify/require"
)

func TestDiskBackedStorage(t *testing.T) {
	require := require.New(t)

	stagingDir, err := ioutil.TempDir("", "staging")
	require.NoError(err)
	defer os.RemoveAll(stagingDir)

	sealedDir, err := ioutil.TempDir("", "sealed")
	require.NoError(err)
	defer os.RemoveAll(sealedDir)

	ss := NewDiskBackedSectorStore(stagingDir, sealedDir)
	defer ss.destroy()

	access1, err := ss.NewStagingSectorAccess()
	require.NoError(err)
	require.Contains(access1, "staging")

	access2, err := ss.NewSealedSectorAccess()
	require.NoError(err)
	require.Contains(access2, "sealed")
}
