package splitstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestNewSplitstore(t *testing.T) {
	tempDir := t.TempDir()
	tempBlocks := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
		newBlock("b4"),
		newBlock("b4"),
	}

	for i, b := range tempBlocks {
		storePath := fmt.Sprintf("base_%d_%s.db", 10+i, b.Cid())
		storePath = filepath.Join(tempDir, storePath)
		err := os.MkdirAll(storePath, 0777)
		require.NoError(t, err)
	}

	ss, err := NewSplitstore(tempDir, nil)
	require.NoError(t, err)
	require.Len(t, ss.layers, ss.maxLayerCount)
}

func TestScan(t *testing.T) {
	tempDir := t.TempDir()
	tempBlocks := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}

	for i, b := range tempBlocks {
		s, err := newLayer(tempDir, int64(10+i), b.Cid())
		require.NoError(t, err)
		if s, ok := s.Blockstore.(Closer); ok {
			err := s.Close()
			require.NoError(t, err)
		}
	}

	// base_init.db(place holder)
	_, err := newLayer(tempDir, int64(0), cid.Undef)
	require.NoError(t, err)

	// any.db will not be scanned in
	err = os.MkdirAll(filepath.Join(tempDir, "any.db"), 0777)
	require.NoError(t, err)

	bs, err := scan(tempDir)
	require.NoError(t, err)

	t.Run("scan in", func(t *testing.T) {
		require.Len(t, bs, len(tempBlocks)+1)

		for i, b := range tempBlocks {
			require.Equal(t, b.Cid(), bs[i+1].Base())
		}

		// store from place holder should be empty
		require.Nil(t, bs[0].Blockstore)
	})

	t.Run("clean up", func(t *testing.T) {
		for i := range bs {
			store := bs[i]
			err := store.Clean()
			require.NoError(t, err)
		}

		bs, err = scan(tempDir)
		require.NoError(t, err)
		require.Len(t, bs, 0)
	})
}

func TestExtractHeightAndCid(t *testing.T) {
	h, _, err := extractHeightAndCid("base_10_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6.db")
	require.NoError(t, err)
	require.Equal(t, int64(10), h)

	_, _, err = extractHeightAndCid("base_10_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6")
	require.Error(t, err)

	_, _, err = extractHeightAndCid("base_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6")
	require.Error(t, err)

	_, _, err = extractHeightAndCid("base_10_bafy2bzacedyokdqa4mnkercuk5hcufi52w5q2xannm567ij2njiqovgwiicx6.db.del")
	require.Error(t, err)
}

func TestScann(t *testing.T) {
	path := "/root/tanlang/docker/test/splitstore/.venus/root/.venus1/splitstore"
	bs, err := scan(path)
	require.NoError(t, err)
	require.Len(t, bs, 2)
}
