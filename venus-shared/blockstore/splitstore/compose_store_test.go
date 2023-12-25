package splitstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/filecoin-project/venus/venus-shared/blockstore"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/require"
)

func TestNewComposeStore(t *testing.T) {
	s := NewComposeStore(nil, nil)
	require.True(t, s.(*ComposeStore).shouldSync)

	s = NewComposeStore(nil, nil, nil)
	require.True(t, s.(*ComposeStore).shouldSync)
	require.False(t, s.(*ComposeStore).secondary.(*ComposeStore).shouldSync)
	require.Nil(t, s.(*ComposeStore).secondary.(*ComposeStore).secondary)
}

func TestComposeStoreGet(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, tertiaryStore := getBlockstore(t)
	blocksInPrimary := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}
	blocksInSecondary := []blocks.Block{
		newBlock("b3"),
		newBlock("b4"),
	}
	blocksInTertiary := []blocks.Block{
		newBlock("b4"),
		newBlock("b5"),
	}
	blockNotExist := newBlock("b6")

	for _, b := range blocksInPrimary {
		require.NoError(t, primaryStore.Put(ctx, b))
	}
	for _, b := range blocksInSecondary {
		require.NoError(t, secondaryStore.Put(ctx, b))
	}
	for _, b := range blocksInTertiary {
		require.NoError(t, tertiaryStore.Put(ctx, b))
	}

	t.Run("Get", func(t *testing.T) {
		for _, b := range blocksInPrimary {
			block, err := composeStore.Get(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, b.RawData(), block.RawData())
		}
		for _, b := range blocksInSecondary {
			block, err := composeStore.Get(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, b.RawData(), block.RawData())
		}
		for _, b := range blocksInTertiary {
			block, err := composeStore.Get(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, b.RawData(), block.RawData())
		}

		_, err := composeStore.Get(ctx, blockNotExist.Cid())
		require.True(t, ipld.IsNotFound(err))

		// test for sync
		// wait for sync (switch goroutine)
		time.Sleep(5 * time.Millisecond)
		for _, b := range blocksInTertiary {
			block, err := primaryStore.Get(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, b.RawData(), block.RawData())
		}
	})
}

func TestComposeStoreGetSize(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, _ := getBlockstore(t)
	blocksInPrimary := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}
	blocksInSecondary := []blocks.Block{
		newBlock("b3"),
		newBlock("b4"),
	}
	blockNotExist := newBlock("b5")

	for _, b := range blocksInPrimary {
		require.NoError(t, primaryStore.Put(ctx, b))
	}
	for _, b := range blocksInSecondary {
		require.NoError(t, secondaryStore.Put(ctx, b))
	}

	t.Run("GetSize", func(t *testing.T) {
		for _, b := range blocksInPrimary {
			sz, err := composeStore.GetSize(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, len(b.RawData()), sz)
		}
		for _, b := range blocksInSecondary {
			sz, err := composeStore.GetSize(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, len(b.RawData()), sz)
		}

		_, err := composeStore.GetSize(ctx, blockNotExist.Cid())
		require.True(t, ipld.IsNotFound(err))

		// test for sync
		// wait for sync (switch goroutine)
		time.Sleep(1 * time.Millisecond)
		for _, b := range blocksInSecondary {
			sz, err := primaryStore.GetSize(ctx, b.Cid())
			require.NoError(t, err)
			require.Equal(t, len(b.RawData()), sz)
		}
	})
}

func TestComposeStoreView(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, _ := getBlockstore(t)
	blocksInPrimary := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}
	blocksInSecondary := []blocks.Block{
		newBlock("b3"),
		newBlock("b4"),
	}
	blockNotExist := newBlock("b5")

	for _, b := range blocksInPrimary {
		require.NoError(t, primaryStore.Put(ctx, b))
	}
	for _, b := range blocksInSecondary {
		require.NoError(t, secondaryStore.Put(ctx, b))
	}

	t.Run("View", func(t *testing.T) {
		for _, b := range blocksInPrimary {
			err := composeStore.View(ctx, b.Cid(), func(bByte []byte) error {
				require.Equal(t, b.RawData(), bByte)
				return nil
			})
			require.NoError(t, err)
		}
		for _, b := range blocksInSecondary {
			err := composeStore.View(ctx, b.Cid(), func(bByte []byte) error {
				require.Equal(t, b.RawData(), bByte)
				return nil
			})
			require.NoError(t, err)
		}

		err := composeStore.View(ctx, blockNotExist.Cid(), func(b []byte) error {
			require.Nil(t, b)
			return nil
		})
		require.True(t, ipld.IsNotFound(err))

		// test for sync
		for _, b := range blocksInSecondary {
			err := composeStore.View(ctx, b.Cid(), func(bByte []byte) error {
				require.Equal(t, b.RawData(), bByte)
				return nil
			})
			require.NoError(t, err)
		}
	})
}

func TestComposeStoreHas(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, _ := getBlockstore(t)
	blocksInPrimary := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}
	blocksInSecondary := []blocks.Block{
		newBlock("b3"),
		newBlock("b4"),
	}
	blockNotExist := newBlock("b5")

	for _, b := range blocksInPrimary {
		require.NoError(t, primaryStore.Put(ctx, b))
	}
	for _, b := range blocksInSecondary {
		require.NoError(t, secondaryStore.Put(ctx, b))
	}

	t.Run("Has", func(t *testing.T) {
		for _, b := range blocksInPrimary {
			h, err := composeStore.Has(ctx, b.Cid())
			require.NoError(t, err)
			require.True(t, h)
		}
		for _, b := range blocksInSecondary {
			h, err := composeStore.Has(ctx, b.Cid())
			require.NoError(t, err)
			require.True(t, h)
		}

		h, err := composeStore.Has(ctx, blockNotExist.Cid())
		require.NoError(t, err)
		require.False(t, h)

		// test for sync
		// wait for sync (switch goroutine)
		time.Sleep(1 * time.Millisecond)
		for _, b := range blocksInSecondary {
			h, err := primaryStore.Has(ctx, b.Cid())
			require.NoError(t, err)
			require.True(t, h)
		}
	})
}

func TestComposeStoreAllKeysChan(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, tertiaryStore := getBlockstore(t)
	blocksInPrimary := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}
	blocksInSecondary := []blocks.Block{
		newBlock("b3"),
		newBlock("b4"),
	}
	blocksInTertiary := []blocks.Block{
		newBlock("b5"),
		newBlock("b6"),
	}

	for _, b := range blocksInPrimary {
		require.NoError(t, primaryStore.Put(ctx, b))
	}
	for _, b := range blocksInSecondary {
		require.NoError(t, secondaryStore.Put(ctx, b))
	}
	for _, b := range blocksInTertiary {
		require.NoError(t, tertiaryStore.Put(ctx, b))
	}

	t.Run("All keys chan", func(t *testing.T) {
		ch, err := composeStore.AllKeysChan(ctx)
		require.NoError(t, err)
		require.NotNil(t, ch)

		cidGet := cid.NewSet()
		for cid := range ch {
			require.True(t, cidGet.Visit(cid))
		}
		for _, b := range blocksInPrimary {
			require.False(t, cidGet.Has(b.Cid()))
		}
		for _, b := range blocksInSecondary {
			require.False(t, cidGet.Has(b.Cid()))
		}

		require.Equal(t, 6, cidGet.Len())
	})
}

func TestComposeStorePut(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, _ := getBlockstore(t)
	blockNotExist := newBlock("b5")

	t.Run("Put", func(t *testing.T) {
		require.NoError(t, composeStore.Put(ctx, blockNotExist))
		h, err := composeStore.Has(ctx, blockNotExist.Cid())
		require.NoError(t, err)
		require.True(t, h)

		h, err = primaryStore.Has(ctx, blockNotExist.Cid())
		require.NoError(t, err)
		require.True(t, h)

		h, err = secondaryStore.Has(ctx, blockNotExist.Cid())
		require.NoError(t, err)
		require.False(t, h)
	})
}

func TestComposeStoreDelete(t *testing.T) {
	ctx := context.Background()
	composeStore, primaryStore, secondaryStore, tertiaryStore := getBlockstore(t)
	blocksInPrimary := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}
	blocksInSecondary := []blocks.Block{
		newBlock("b3"),
		newBlock("b4"),
	}
	blocksInTertiary := []blocks.Block{
		newBlock("b3"),
		newBlock("b6"),
	}
	blockNotExist := newBlock("b5")

	for _, b := range blocksInPrimary {
		require.NoError(t, primaryStore.Put(ctx, b))
	}
	for _, b := range blocksInSecondary {
		require.NoError(t, secondaryStore.Put(ctx, b))
	}
	for _, b := range blocksInTertiary {
		require.NoError(t, tertiaryStore.Put(ctx, b))
	}

	t.Run("Delete", func(t *testing.T) {
		for _, b := range blocksInPrimary {
			err := composeStore.DeleteBlock(ctx, b.Cid())
			require.NoError(t, err)

			h, err := composeStore.Has(ctx, b.Cid())
			require.NoError(t, err)
			require.False(t, h)
		}
		for _, b := range blocksInSecondary {
			err := composeStore.DeleteBlock(ctx, b.Cid())
			require.NoError(t, err)

			h, err := composeStore.Has(ctx, b.Cid())
			require.NoError(t, err)
			require.False(t, h)
		}

		err := composeStore.DeleteBlock(ctx, blockNotExist.Cid())
		require.NoError(t, err)
	})
}

func getBlockstore(t *testing.T) (compose, primary, secondary, tertiary blockstore.Blockstore) {
	tempDir := t.TempDir()

	primaryPath := filepath.Join(tempDir, "primary")
	secondaryPath := filepath.Join(tempDir, "secondary")
	tertiaryPath := filepath.Join(tempDir, "tertiary")

	optPri, err := blockstore.BadgerBlockstoreOptions(primaryPath, false)
	require.NoError(t, err)
	dsPri, err := blockstore.Open(optPri)
	require.NoError(t, err)

	optSnd, err := blockstore.BadgerBlockstoreOptions(secondaryPath, false)
	require.NoError(t, err)
	dsSnd, err := blockstore.Open(optSnd)
	require.NoError(t, err)

	optTertiary, err := blockstore.BadgerBlockstoreOptions(tertiaryPath, false)
	require.NoError(t, err)
	dsTertiary, err := blockstore.Open(optTertiary)
	require.NoError(t, err)

	return NewComposeStore(dsTertiary, dsSnd, dsPri), dsPri, dsSnd, dsTertiary
}

func newBlock(s string) blocks.Block {
	return blocks.NewBlock([]byte(s))
}
