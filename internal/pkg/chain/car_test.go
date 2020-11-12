package chain_test

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-amt-ipld/v2"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	typegen "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

func TestChainImportExportGenesis(t *testing.T) {
	tf.UnitTest(t)

	ctx, gene, cb, carW, carR, bstore := setupDeps(t)

	// export the car file to a carW
	mustExportToBuffer(ctx, t, gene, cb, &mockStateReader{}, carW)

	// import the car file from the carR
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, gene.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, gene.Key(), gene.Key(), bstore)
}

func TestChainImportExportSingleTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one
	headTS := cb.AppendOn(gene, 1)

	// export the car file to carW
	mustExportToBuffer(ctx, t, headTS, cb, &mockStateReader{}, carW)

	// import the car file from carR
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportWideTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one, two wide
	headTS := cb.AppendOn(gene, 2)
	// export the car file to a carW
	mustExportToBuffer(ctx, t, headTS, cb, &mockStateReader{}, carW)
	// import the car file from carR
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)
	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one
	headTS := cb.AppendOn(gene, 1)
	headTS = cb.AppendOn(headTS, 1)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, &mockStateReader{}, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiWideTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one
	headTS := cb.AppendOn(gene, 1)
	// extend by one, two wide.
	headTS = cb.AppendOn(headTS, 2)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, &mockStateReader{}, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiWideBaseTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one, two wide
	headTS := cb.AppendOn(gene, 2)
	// extend by one
	headTS = cb.AppendOn(headTS, 1)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, &mockStateReader{}, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiWideTips(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one, two wide
	headTS := cb.AppendOn(gene, 2)
	// extend by one, two wide
	headTS = cb.AppendOn(headTS, 2)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, &mockStateReader{}, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMessages(t *testing.T) {
	tf.UnitTest(t)

	ctx, gene, cb, carW, carR, bstore := setupDeps(t)

	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]

	ts1 := cb.AppendManyOn(1, gene)
	msgs := []*types.SignedMessage{
		mm.NewSignedMessage(alice, 1),
		mm.NewSignedMessage(alice, 2),
		mm.NewSignedMessage(alice, 3),
		mm.NewSignedMessage(alice, 4),
		mm.NewSignedMessage(alice, 5),
	}
	ts2 := cb.BuildOneOn(ts1, func(b *chain.BlockBuilder) {
		b.AddMessages(msgs, []*types.UnsignedMessage{})
	})

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, ts2, cb, &mockStateReader{}, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, ts2.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, ts2.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiTipSetWithMessages(t *testing.T) {
	tf.UnitTest(t)

	ctx, gene, cb, carW, carR, bstore := setupDeps(t)

	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]

	ts1 := cb.AppendManyOn(1, gene)
	msgs := []*types.SignedMessage{
		mm.NewSignedMessage(alice, 1),
		mm.NewSignedMessage(alice, 2),
		mm.NewSignedMessage(alice, 3),
		mm.NewSignedMessage(alice, 4),
		mm.NewSignedMessage(alice, 5),
	}
	ts2 := cb.BuildOneOn(ts1, func(b *chain.BlockBuilder) {
		b.AddMessages(
			msgs,
			[]*types.UnsignedMessage{},
		)
	})

	ts3 := cb.AppendOn(ts2, 3)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, ts3, cb, &mockStateReader{}, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, ts3.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(ctx, t, ts3.Key(), gene.Key(), bstore)
}

func mustExportToBuffer(ctx context.Context, t *testing.T, head *block.TipSet, cb *chain.Builder, msr *mockStateReader, carW *bufio.Writer) {
	err := chain.Export(ctx, head, cb, cb, msr, carW)
	assert.NoError(t, err)
	require.NoError(t, carW.Flush())
}

func mustImportFromBuffer(ctx context.Context, t *testing.T, bstore blockstore.Blockstore, carR *bufio.Reader) block.TipSetKey {
	importedKey, err := chain.Import(ctx, bstore, carR)
	assert.NoError(t, err)
	return importedKey
}

func setupDeps(t *testing.T) (context.Context, *block.TipSet, *chain.Builder, *bufio.Writer, *bufio.Reader, blockstore.Blockstore) {
	// context for operations
	ctx := context.Background()

	// chain builder and its genesis
	cb := chain.NewBuilder(t, address.Undef)
	// buffers to read and write the car file from
	var buf bytes.Buffer
	carW := bufio.NewWriter(&buf)
	carR := bufio.NewReader(&buf)

	// a store to import the car file to and validate from.
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	return ctx, cb.Genesis(), cb, carW, carR, bstore

}

func validateBlockstoreImport(ctx context.Context, t *testing.T, start, stop block.TipSetKey, bstore blockstore.Blockstore) {
	as := cbor.NewCborStore(bstore)

	// walk the blockstore and assert it had all blocks imported
	cur := start
	for {
		var parents block.TipSetKey
		for _, c := range cur.ToSlice() {
			bsBlk, err := bstore.Get(c)
			assert.NoError(t, err)
			blk, err := block.DecodeBlock(bsBlk.RawData())
			assert.NoError(t, err)

			txMetaBlk, err := bstore.Get(blk.Messages.Cid)
			require.NoError(t, err)
			var meta types.TxMeta
			require.NoError(t, encoding.Decode(txMetaBlk.RawData(), &meta))

			secpAMT, err := amt.LoadAMT(ctx, as, meta.SecpRoot.Cid)
			require.NoError(t, err)

			var smsg types.SignedMessage
			requireAMTDecoding(ctx, t, bstore, secpAMT, &smsg)

			blsAMT, err := amt.LoadAMT(ctx, as, meta.BLSRoot.Cid)
			require.NoError(t, err)

			var umsg types.UnsignedMessage
			requireAMTDecoding(ctx, t, bstore, blsAMT, &umsg)

			rectAMT, err := amt.LoadAMT(ctx, as, blk.MessageReceipts.Cid)
			require.NoError(t, err)

			var rect types.MessageReceipt
			requireAMTDecoding(ctx, t, bstore, rectAMT, &rect)

			if parents.Len() == 0 {
				parents = blk.Parents
			} else {
				assert.True(t, blk.Parents.Equals(parents), "malformed tipsets in imported chain")
			}
		}
		if cur.Equals(stop) {
			break
		}
		if cur.Equals(parents) {
			t.Fatal("validate blockstore import is looping")
		}
		cur = parents
	}
}

func requireAMTDecoding(ctx context.Context, t *testing.T, bstore blockstore.Blockstore, root *amt.Root, dest interface{}) {
	err := root.ForEach(ctx, func(_ uint64, d *typegen.Deferred) error {
		var c enccid.Cid
		if err := encoding.Decode(d.Raw, &c); err != nil {
			return err
		}

		b, err := bstore.Get(c.Cid)
		if err != nil {
			return err
		}
		return encoding.Decode(b.RawData(), dest)
	})
	require.NoError(t, err)

}

type mockStateReader struct{}

func (mr *mockStateReader) ChainStateTree(ctx context.Context, c cid.Cid) ([]format.Node, error) {
	return nil, nil
}
