package splitstore

import (
	"context"
	"fmt"
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
	opt.Prefix = bstore.BlockPrefix.String()
	if err != nil {
		return nil, err
	}
	return blockstore.Open(opt)
}

// todo: remove these test cases
func TestTsk(t *testing.T) {
	bCid := cid.MustParse("bafy2bzacecszjrdivfsgbxetthe7uac56wjvstrk4qewxsvefes3xgdi6lbmc")
	tskCid, err := types.NewTipSetKey(bCid).Cid()
	require.NoError(t, err)

	c := cid.MustParse("bafy2bzacedm5ygprszgbimh6ymuuwkv3xegkpjlmuu5omm4tviftpubvda7ha")

	ds, err := openStore("/root/tanlang/docker/test/splitstore/.venus/root/.venus/splitstore/base_42_bafy2bzacedm5ygprszgbimh6ymuuwkv3xegkpjlmuu5omm4tviftpubvda7ha.db")
	require.NoError(t, err)

	has, err := ds.Has(context.Background(), c)
	require.NoError(t, err)

	fmt.Printf("has: %v\n", has)
	fmt.Printf("tskCid: %v %v\n", tskCid, c)
}

func TestIterSomeKey(t *testing.T) {
	ds, err := openStore("/root/tanlang/docker/test/splitstore/.venus/root/.venus/splitstore/base_3726_bafy2bzaced65gadcvjd4fi4orxjdnetmvguqbyozsqbncpokomtbseejou4yw.db")
	require.NoError(t, err)

	ch, err := ds.AllKeysChan(context.Background())
	require.NoError(t, err)

	for c := range ch {
		fmt.Printf("cid: %v\n", c)
	}
}

func TestCid(t *testing.T) {
	_ = cid.MustParse("bafk2bzacedulsjdqqedrf5wvkljo7dsold3gf4ve7ljqgdqibpnhnpjdmg4s4")
}
