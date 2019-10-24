package chain_test

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/block"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainImportExportGenesis(t *testing.T) {
	tf.UnitTest(t)

	ctx, gene, cb, carW, carR, bstore := setupDeps(t)

	// export the car file to a carW
	mustExportToBuffer(ctx, t, gene, cb, carW)

	// import the car file from the carR
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, gene.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, gene.Key(), gene.Key(), bstore)
}

func TestChainImportExportSingleTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one
	headTS := cb.AppendOn(gene, 1)

	// export the car file to carW
	mustExportToBuffer(ctx, t, headTS, cb, carW)

	// import the car file from carR
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportWideTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one, two wide
	headTS := cb.AppendOn(gene, 2)

	// export the car file to a carW
	mustExportToBuffer(ctx, t, headTS, cb, carW)

	// import the car file from carR
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one
	headTS := cb.AppendOn(gene, 1)
	headTS = cb.AppendOn(headTS, 1)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiWideTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one
	headTS := cb.AppendOn(gene, 1)
	// extend by one, two wide.
	headTS = cb.AppendOn(headTS, 2)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiWideBaseTip(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one, two wide
	headTS := cb.AppendOn(gene, 2)
	// extend by one
	headTS = cb.AppendOn(headTS, 1)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, headTS.Key(), gene.Key(), bstore)
}

func TestChainImportExportMultiWideTips(t *testing.T) {
	tf.UnitTest(t)
	ctx, gene, cb, carW, carR, bstore := setupDeps(t)
	// extend the head by one, two wide
	headTS := cb.AppendOn(gene, 2)
	// extend by one, two wide
	headTS = cb.AppendOn(headTS, 2)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, headTS, cb, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, headTS.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, headTS.Key(), gene.Key(), bstore)
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
	rcts := types.EmptyReceipts(5)
	ts2 := cb.BuildOneOn(ts1, func(b *chain.BlockBuilder) {
		b.AddMessages(msgs, []*types.UnsignedMessage{}, rcts)
	})

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, ts2, cb, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, ts2.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, ts2.Key(), gene.Key(), bstore)
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
	rcts := types.EmptyReceipts(5)
	ts2 := cb.BuildOneOn(ts1, func(b *chain.BlockBuilder) {
		b.AddMessages(
			msgs,
			[]*types.UnsignedMessage{},
			rcts,
		)
	})

	ts3 := cb.AppendOn(ts2, 3)

	// export the car file to a buffer
	mustExportToBuffer(ctx, t, ts3, cb, carW)

	// import the car file from the buffer
	importedKey := mustImportFromBuffer(ctx, t, bstore, carR)
	assert.Equal(t, ts3.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	validateBlockstoreImport(t, ts3.Key(), gene.Key(), bstore)
}

func mustExportToBuffer(ctx context.Context, t *testing.T, head block.TipSet, cb *chain.Builder, carW *bufio.Writer) {
	err := chain.Export(ctx, head, cb, cb, carW)
	assert.NoError(t, err)
	require.NoError(t, carW.Flush())
}

func mustImportFromBuffer(ctx context.Context, t *testing.T, bstore blockstore.Blockstore, carR *bufio.Reader) block.TipSetKey {
	importedKey, err := chain.Import(ctx, bstore, carR)
	assert.NoError(t, err)
	return importedKey
}

func setupDeps(t *testing.T) (context.Context, block.TipSet, *chain.Builder, *bufio.Writer, *bufio.Reader, blockstore.Blockstore) {
	// context for operations
	ctx := context.Background()

	// chain builder and its genesis
	cb := chain.NewBuilder(t, address.Undef)
	gene := cb.NewGenesis()
	// buffers to read and write the car file from
	var buf bytes.Buffer
	carW := bufio.NewWriter(&buf)
	carR := bufio.NewReader(&buf)

	// a store to import the car file to and validate from.
	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)
	return ctx, gene, cb, carW, carR, bstore

}

func validateBlockstoreImport(t *testing.T, start, stop block.TipSetKey, bstore blockstore.Blockstore) {
	// walk the blockstore and assert it had all blocks imported
	cur := start
	for {
		var parents []cid.Cid
		for _, c := range cur.ToSlice() {
			bsBlk, err := bstore.Get(c)
			assert.NoError(t, err)
			blk, err := block.DecodeBlock(bsBlk.RawData())
			assert.NoError(t, err)

			bsSecpMsgs, err := bstore.Get(blk.Messages.SecpRoot)
			assert.NoError(t, err)
			_, err = types.DecodeSignedMessages(bsSecpMsgs.RawData())
			assert.NoError(t, err)

			bsBlsMsgs, err := bstore.Get(blk.Messages.BLSRoot)
			assert.NoError(t, err)
			_, err = types.DecodeMessages(bsBlsMsgs.RawData())
			assert.NoError(t, err)

			bsRcts, err := bstore.Get(blk.MessageReceipts)
			assert.NoError(t, err)
			_, err = types.DecodeReceipts(bsRcts.RawData())
			assert.NoError(t, err)

			for _, p := range blk.Parents.ToSlice() {
				parents = append(parents, p)
			}
		}
		if cur.Equals(stop) {
			break
		}
		cur = block.NewTipSetKey(parents...)
	}
}
