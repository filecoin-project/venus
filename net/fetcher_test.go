package net_test

import (
	"context"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	bstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmSz8kAe2JCKp2dWSG8gHSWnwSmne8YfRXTeK5HBmc9L7t/go-ipfs-exchange-offline"
	"gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	dss "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore/sync"
	bserv "gx/ipfs/QmZsGVGCqMCNzHLNMB6q4F6yyvomqf1VxwhJwSfgo1NGaF/go-blockservice"

	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/types"
)

func requireBlockStorePut(require *require.Assertions, bs bstore.Blockstore, data ipld.Node) {
	err := bs.Put(data)
	require.NoError(err)
}

func TestFetchHappyPath(t *testing.T) {
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
