package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	badger "github.com/ipfs/go-ds-badger2"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/vm/storage"
)

func TestBatchSize(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	dir, err := ioutil.TempDir("", "storagetest")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(dir)
	}()
	ds, err := badger.NewDatastore(dir, &badger.DefaultOptions)
	require.NoError(t, err)
	bs := blockstore.NewBlockstore(ds)
	store := storage.NewStorage(bs)

	// This iteration count was picked experimentally based on a badger default maxtablesize of 64 << 20.
	// If the batching is disabled inside the store, this test should fail.
	require.Equal(t, int64(16<<20), badger.DefaultOptions.MaxTableSize)
	iterCount := int64(2) << 16

	data := bytes.Repeat([]byte("badger"), 100)
	for i := int64(0); i < iterCount; i++ {
		_, _, err = store.PutWithLen(ctx, fmt.Sprintf("%s%d", data, i))
		require.NoError(t, err)
	}
	err = store.Flush()
	require.NoError(t, err)
}
