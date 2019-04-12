package net_test

import (
	"context"
	"testing"

	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/net"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func requireBlockStorePut(require *require.Assertions, bs bstore.Blockstore, data ipld.Node) {
	err := bs.Put(data)
	require.NoError(err)
}

func TestFetchHappyPath(t *testing.T) {
	tf.UnitTest(t)

	require := require.New(t)
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	fetcher := net.NewFetcher(context.Background(), bserv.New(bs, offline.Exchange(bs)))
	block1 := types.NewBlockForTest(nil, uint64(0))
	block2 := types.NewBlockForTest(nil, uint64(1))
	block3 := types.NewBlockForTest(nil, uint64(3))

	requireBlockStorePut(require, bs, block1.ToNode())
	requireBlockStorePut(require, bs, block2.ToNode())
	requireBlockStorePut(require, bs, block3.ToNode())
	originalCids := types.NewSortedCidSet(block1.Cid(), block2.Cid(), block3.Cid())

	fetchedBlocks, err := fetcher.GetBlocks(context.Background(), originalCids.ToSlice())
	require.NoError(err)
	require.Equal(3, len(fetchedBlocks))
	fetchedCids := types.NewSortedCidSet(
		fetchedBlocks[0].Cid(),
		fetchedBlocks[1].Cid(),
		fetchedBlocks[2].Cid(),
	)

	require.True(originalCids.Equals(fetchedCids))
}

func TestFetchNoBlockFails(t *testing.T) {
	tf.UnitTest(t)

	require := require.New(t)
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	fetcher := net.NewFetcher(context.Background(), bserv.New(bs, offline.Exchange(bs)))
	block1 := types.NewBlockForTest(nil, uint64(0))
	block2 := types.NewBlockForTest(nil, uint64(1))

	// do not add block2 to the bstore
	requireBlockStorePut(require, bs, block1.ToNode())
	cids := types.NewSortedCidSet(block1.Cid(), block2.Cid())

	blocks, err := fetcher.GetBlocks(context.Background(), cids.ToSlice())
	require.Error(err)
	require.Nil(blocks)
}

func TestFetchNotBlockFormat(t *testing.T) {
	tf.UnitTest(t)

	require := require.New(t)
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	fetcher := net.NewFetcher(context.Background(), bserv.New(bs, offline.Exchange(bs)))
	notABlock := types.NewMsgs(1)[0]
	notABlockObj, err := notABlock.ToNode()
	require.NoError(err)

	requireBlockStorePut(require, bs, notABlockObj)
	notABlockCid, err := notABlock.Cid()
	require.NoError(err)

	blocks, err := fetcher.GetBlocks(context.Background(), []cid.Cid{notABlockCid})
	require.Error(err)
	require.Nil(blocks)
}
