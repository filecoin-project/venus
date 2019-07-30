package net_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestGraphsyncFetcherHappyPath(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	// setup a chain
	builder := chain.NewBuilder(t, address.Undef)
	gen := builder.NewGenesis()
	final := builder.AppendManyOn(30, gen)

	// setup network
	mn := mocknet.New(ctx)

	host1, err := mn.GenPeer()
	if err != nil {
		t.Fatal("error generating host")
	}
	host2, err := mn.GenPeer()
	if err != nil {
		t.Fatal("error generating host")
	}
	err = mn.LinkAll()
	if err != nil {
		t.Fatal("error linking hosts")
	}

	gsnet1 := gsnet.NewFromLibp2pHost(host1)

	// setup receiving peer to just record message coming in
	gsnet2 := gsnet.NewFromLibp2pHost(host2)

	// setup a graphsync fetcher and a graphsync responder

	bridge1 := ipldbridge.NewIPLDBridge()
	bridge2 := ipldbridge.NewIPLDBridge()
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	bv := th.NewFakeBlockValidator()

	localLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		asCidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("Unsupported Link Type")
		}
		block, err := bs.Get(asCidLink.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(block.RawData()), nil
	}

	localStorer := func(lnkCtx ipld.LinkContext) (io.Writer, ipld.StoreCommitter, error) {
		var buffer bytes.Buffer
		committer := func(lnk ipld.Link) error {
			asCidLink, ok := lnk.(cidlink.Link)
			if !ok {
				return fmt.Errorf("Unsupported Link Type")
			}
			block, err := blocks.NewBlockWithCid(buffer.Bytes(), asCidLink.Cid)
			if err != nil {
				return err
			}
			return bs.Put(block)
		}
		return &buffer, committer, nil
	}

	localGraphsync := graphsync.New(ctx, gsnet1, bridge1, localLoader, localStorer)
	fetcher := net.NewGraphSyncFetcher(ctx, localGraphsync, bs, bv)

	remoteLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		block, err := builder.GetBlock(ctx, cid)
		if err != nil {
			return nil, err
		}
		raw, err := cbor.DumpObject(block)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(raw), nil
	}
	graphsync.New(ctx, gsnet2, bridge2, remoteLoader, nil)

	tipsets, err := fetcher.FetchTipSets(ctx, final.Key(), host2.ID(), func(ts types.TipSet) (bool, error) {
		if ts.Key().Equals(gen.Key()) {
			return true, nil
		}
		return false, nil
	})

	require.NoError(t, err)

	require.Equal(t, 31, len(tipsets))

	for _, ts := range tipsets {
		matchedTs, err := builder.GetTipSet(ts.Key())
		require.NoError(t, err)
		require.NotNil(t, matchedTs)
	}
}
