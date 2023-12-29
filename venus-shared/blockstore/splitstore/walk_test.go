package splitstore

import (
	"context"
	"testing"

	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/logging"
	"github.com/filecoin-project/venus/venus-shared/types"
	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/stretchr/testify/require"
)

func init() {
	err := logging.SetLogLevel("splitstore", "debug")
	if err != nil {
		panic(err)
	}
}

func TestWalk(t *testing.T) {
	ctx := context.Background()

	log.Info("log level")
	log.Debug("log level")

	badgerPath := "./test_data/base_583_bafy2bzaceazuutcexhvwkyyesszohrkjjzk2zgknasgs7bb7zgfnwghtnu5w2.db"
	blockCid := cid.MustParse("bafy2bzaceazuutcexhvwkyyesszohrkjjzk2zgknasgs7bb7zgfnwghtnu5w2")

	ds, err := openStore(badgerPath)
	require.NoError(t, err)

	cst := cbor.NewCborStore(ds)

	var b types.BlockHeader
	err = cst.Get(ctx, blockCid, &b)
	require.NoError(t, err)

	tsk := types.NewTipSetKey(blockCid)
	require.False(t, tsk.IsEmpty())

	tskCid, err := tsk.Cid()
	require.NoError(t, err)

	err = WalkUntil(ctx, ds, tskCid, 10)
	require.NoError(t, err)
}

func TestVisitor(t *testing.T) {
	t.Run("visit duplicated cid", func(t *testing.T) {
		v := NewSyncVisitor()
		c, err := types.DefaultCidBuilder.Sum([]byte("duplicated_cid"))
		require.NoError(t, err)
		require.True(t, v.Visit(c))
		require.False(t, v.Visit(c))
		require.Equal(t, v.Len(), 1)
		require.Len(t, v.Cids(), 1)
		require.Equal(t, v.Cids()[0], c)
	})

	t.Run("test hook", func(t *testing.T) {
		v := NewSyncVisitor()
		c, err := types.DefaultCidBuilder.Sum([]byte("cids_reject"))
		require.NoError(t, err)
		v.RegisterVisitHook(func(a cid.Cid) bool {
			return !a.Equals(c)
		})

		require.False(t, v.Visit(c))
	})
}

func openStore(path string) (*blockstore.BadgerBlockstore, error) {
	opt, err := blockstore.BadgerBlockstoreOptions(path, false)
	opt.Prefix = bstore.BlockPrefix.String()
	if err != nil {
		return nil, err
	}
	return blockstore.Open(opt)
}
