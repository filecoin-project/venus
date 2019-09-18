package chain_test

import (
	"bufio"
	"bytes"
	"context"
	"testing"

	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestChainImportExportSimple(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	cb := chain.NewBuilder(t, address.Undef)

	gene := cb.NewGenesis()
	ts10 := cb.AppendManyOn(9, gene)

	var buf bytes.Buffer
	carW := bufio.NewWriter(&buf)

	// export the car file to a buffer
	exportedKey, err := chain.Export(ctx, ts10, cb, cb, carW)
	assert.NoError(t, err)
	assert.Equal(t, ts10.Key(), exportedKey)
	require.NoError(t, carW.Flush())

	mds := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(mds)

	// import the car file from the buffer
	carR := bufio.NewReader(&buf)
	importedKey, err := chain.Import(ctx, bstore, carR)
	assert.NoError(t, err)
	assert.Equal(t, ts10.Key(), importedKey)

	// walk the blockstore and assert it had all blocks imported
	cur := ts10.At(0).Cid()
	for !cur.Equals(gene.At(0).Cid()) {
		bsBlk, err := bstore.Get(cur)
		assert.NoError(t, err)
		blk, err := types.DecodeBlock(bsBlk.RawData())
		cur = blk.Parents.ToSlice()[0]
	}
}
