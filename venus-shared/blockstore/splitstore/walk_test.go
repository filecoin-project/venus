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

	seen := NewSyncVisitor()

	err = WalkChain(ctx, ds, tskCid, seen, 14)
	require.NoError(t, err)
}

func openStore(path string) (*blockstore.BadgerBlockstore, error) {
	opt, err := blockstore.BadgerBlockstoreOptions(path, false)
	opt.Prefix = bstore.BlockPrefix.String()
	if err != nil {
		return nil, err
	}
	return blockstore.Open(opt)
}
