package splitstore

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/logging"
	"github.com/filecoin-project/venus/venus-shared/types"
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

	badgerPath := "./test_data/test_20615_bafy2bzacea53rxdtdrsaovap3bsfpash2sx2cu5ho2unoxg24kl2z3opcjjda"
	blockCid := cid.MustParse("bafy2bzacea53rxdtdrsaovap3bsfpash2sx2cu5ho2unoxg24kl2z3opcjjda")

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
	opt.Prefix = "blocks"
	if err != nil {
		return nil, err
	}
	return blockstore.Open(opt)
}

func TestGetAllKeys(t *testing.T) {
	// 打开数据库
	badgerPath := "/root/tanlang/venus/.vscode/venus_16391_bafy2bzacebdffckqhdnvm767hon3osuv77iryvcjfcvtob6sm2vz6je346k5o"
	db, err := badger.Open(badger.DefaultOptions(badgerPath))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 创建一个事务
	txn := db.NewTransaction(false)
	defer txn.Discard()

	// 迭代遍历所有的 keys
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 10
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Rewind(); it.Valid(); it.Next() {
		item := it.Item()
		key := item.KeyCopy(nil)
		fmt.Println(string(key))
	}
}

func TestRand(t *testing.T) {
	rand.New(rand.NewSource(0))
	rand.Seed(0)
}
