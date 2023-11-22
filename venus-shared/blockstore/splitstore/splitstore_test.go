package splitstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestSplitstore(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	splitstorePath := filepath.Join(tempDir, "splitstore")

	initStore, err := openStore("./test_data/base_583_bafy2bzaceazuutcexhvwkyyesszohrkjjzk2zgknasgs7bb7zgfnwghtnu5w2.db")
	require.NoError(t, err)

	ss, err := NewSplitstore(splitstorePath, initStore)
	require.NoError(t, err)

	blockCid := cid.MustParse("bafy2bzaceazuutcexhvwkyyesszohrkjjzk2zgknasgs7bb7zgfnwghtnu5w2")
	tskCid, err := types.NewTipSetKey(blockCid).Cid()

	// apply head change to append new store
	b, err := ss.getBlock(ctx, blockCid)
	require.NoError(t, err)
	ts, err := types.NewTipSet([]*types.BlockHeader{b})
	require.NoError(t, err)
	ss.HeadChange(nil, []*types.TipSet{ts})
	require.Len(t, ss.stores, 2)

	seenBefore := NewSyncVisitor()
	t.Run("initial walk chain", func(t *testing.T) {
		err = WalkChain(ctx, ss, tskCid, seenBefore, 0)
		require.NoError(t, err)
	})

	seenAfter := NewSyncVisitor()
	t.Run("walk chain after initialize", func(t *testing.T) {
		err = WalkChain(ctx, ss.stores[1], tskCid, seenAfter, 0)
		require.NoError(t, err)
	})

	require.Equal(t, seenBefore.Len(), seenAfter.Len())
}

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
	require.Len(t, ss.stores, ss.maxStoreCount)
}

func TestScan(t *testing.T) {
	tempDir := t.TempDir()
	tempBlocks := []blocks.Block{
		newBlock("b1"),
		newBlock("b2"),
		newBlock("b3"),
	}

	for i, b := range tempBlocks {
		s, err := newInnerStore(tempDir, int64(10+i), b.Cid())
		require.NoError(t, err)
		if s, ok := s.Blockstore.(Closer); ok {
			err := s.Close()
			require.NoError(t, err)
		}
	}

	// base_0_init.db(place holder)
	_, err := newInnerStore(tempDir, int64(0), cid.Undef)
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

	t.Run("slean up", func(t *testing.T) {
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
	h, c, err := extractHeightAndCid("base_10_b1.db")
	require.NoError(t, err)
	require.Equal(t, int64(10), h)
	require.Equal(t, "b1", c)

	h, c, err = extractHeightAndCid("base_10_b1")
	require.Error(t, err)

	h, c, err = extractHeightAndCid("base_b1")
	require.Error(t, err)
}

func fakeTipset(height abi.ChainEpoch) *types.TipSet {
	c, _ := abi.CidBuilder.Sum([]byte("any"))

	bh := &types.BlockHeader{
		Miner:                 address.TestAddress,
		Messages:              c,
		ParentStateRoot:       c,
		ParentMessageReceipts: c,
		Height:                height,
	}

	ts, _ := types.NewTipSet([]*types.BlockHeader{bh})
	return ts
}
