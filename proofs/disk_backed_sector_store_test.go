package proofs

import (
	"testing"

	"io/ioutil"

	"os"

	"github.com/stretchr/testify/require"
)

func TestDiskBackedStorage(t *testing.T) {
	// TODO: This test should be split into several, more finely-grained tests
	// once the DiskBackedStorage does something useful.

	require := require.New(t)

	stagingDir, err := ioutil.TempDir("", "staging")
	require.NoError(err)
	defer os.RemoveAll(stagingDir)

	sealedDir, err := ioutil.TempDir("", "sealed")
	require.NoError(err)
	defer os.RemoveAll(sealedDir)

	ss := NewDiskBackedSectorStore(stagingDir, sealedDir)
	defer ss.destroy()

	// dispense some sector access

	res1, err := ss.NewStagingSectorAccess()
	require.NoError(err)
	require.Contains(res1.SectorAccess, "staging")

	res2, err := ss.NewSealedSectorAccess()
	require.NoError(err)
	require.Contains(res2.SectorAccess, "sealed")

	// write to unsealed area

	res, err := ss.WriteUnsealed(WriteUnsealedRequest{
		SectorAccess: res1.SectorAccess,
		Data:         []byte("hello, moto"),
	})
	require.NoError(err)
	require.Equal(uint64(len([]byte("hello, moto"))), res.NumBytesWritten)

	// bypass the DiskBackedStorage, reading from the filesystem directly to
	// make sure that our implementation works
	//
	// TODO: replace this part of the test with something that the
	// DiskBackedStorage to read from a sector access so we're not testing the
	// implementation of DiskBackedStorage but rather its interface

	bytesWeRead, err := ioutil.ReadFile(res1.SectorAccess)
	require.NoError(err)
	require.Equal("hello, moto", string(bytesWeRead))

	// write more bytes to confirm that we're appending

	res, err = ss.WriteUnsealed(WriteUnsealedRequest{
		SectorAccess: res1.SectorAccess,
		Data:         []byte(" bike"),
	})
	require.NoError(err)
	require.Equal(uint64(len([]byte(" bike"))), res.NumBytesWritten)

	bytesWeRead, err = ioutil.ReadFile(res1.SectorAccess)
	require.NoError(err)
	require.Equal("hello, moto bike", string(bytesWeRead))
}
