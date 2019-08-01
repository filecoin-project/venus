package net_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	ipld "github.com/ipld/go-ipld-prime"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestGraphsyncFetcher(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	bv := th.NewFakeBlockValidator()

	ssb := selector.NewSelectorSpecBuilder(ipldfree.NodeBuilder())
	layer1Selector, err := ssb.Matcher().Selector()
	require.NoError(t, err)
	gsSelector, err := ssb.ExploreRecursive(1, ssb.ExploreFields(func(efsb selector.ExploreFieldsSpecBuilder) {
		efsb.Insert("parents", ssb.ExploreUnion(
			ssb.ExploreAll(ssb.Matcher()),
			ssb.ExploreIndex(0, ssb.ExploreRecursiveEdge()),
		))
	})).Selector()
	require.NoError(t, err)

	t.Run("happy path returns correct tipsets", func(t *testing.T) {
		builder := chain.NewBuilder(t, address.Undef)
		gen := builder.NewGenesis()
		final := builder.AppendOn(gen, 3)

		stubs := []requestResponse{
			{
				fakeRequest{cidlink.Link{Cid: final.At(0).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(0).ToNode()}},
			},
			{
				fakeRequest{cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(1).ToNode()}},
			},
			{
				fakeRequest{cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(2).ToNode()}},
			},
			{fakeRequest{cidlink.Link{Cid: final.At(0).Cid()}, gsSelector}, fakeResponse{
				responses: []graphsync.ResponseProgress{
					makeGsResponse("", final.At(0).Cid()),
					makeGsResponse("parents", final.At(0).Cid()),
					makeGsResponse("parents/0", gen.At(0).Cid()),
				},
				blks: []format.Node{final.At(0).ToNode(), gen.At(0).ToNode()},
			}},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv)

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(gen.Key()) {
				return true, nil
			}
			return false, nil
		}

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), peer.ID("fake"), done)
		require.NoError(t, err, "the request completes successfully")
		require.Equal(t, 4, len(mgs.receivedRequests), "all expected graphsync requests are made")
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("value returned with non block format", func(t *testing.T) {
		notABlock := types.NewMsgs(1)[0]
		notABlockObj, err := notABlock.ToNode()
		require.NoError(t, err)

		notABlockCid, err := notABlock.Cid()
		require.NoError(t, err)
		originalCids := types.NewTipSetKey(notABlockCid)

		stubs := []requestResponse{
			{
				fakeRequest{cidlink.Link{Cid: notABlockCid}, layer1Selector},
				fakeResponse{blks: []format.Node{notABlockObj}},
			},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv)

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(originalCids) {
				return true, nil
			}
			return false, nil
		}
		ts, err := fetcher.FetchTipSets(ctx, originalCids, peer.ID("fake"), done)
		require.Errorf(t, err, "fetched data (cid %s) was not a block", notABlockCid.String())
		require.Nil(t, ts)
	})
}

func TestRealWorldGraphsyncFetchAcrossNetwork(t *testing.T) {
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

type fakeRequest struct {
	root     ipld.Link
	selector selector.Selector
}

type fakeResponse struct {
	responses []graphsync.ResponseProgress
	errs      []error
	blks      []format.Node
}

type requestResponse struct {
	request  fakeRequest
	response fakeResponse
}

type mockableGraphsync struct {
	stubs            []requestResponse
	receivedRequests []fakeRequest
	store            bstore.Blockstore
	t                *testing.T
}

func (mgs *mockableGraphsync) toChans(mr fakeResponse) (<-chan graphsync.ResponseProgress, <-chan error) {
	errChan := make(chan error, len(mr.errs))
	for _, err := range mr.errs {
		errChan <- err
	}
	close(errChan)

	responseChan := make(chan graphsync.ResponseProgress, len(mr.responses))
	for _, response := range mr.responses {
		responseChan <- response
	}
	close(responseChan)

	for _, block := range mr.blks {
		requireBlockStorePut(mgs.t, mgs.store, block)
	}
	return responseChan, errChan
}

func (mgs *mockableGraphsync) Request(ctx context.Context, p peer.ID, root ipld.Link, selectorSpec ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error) {
	parsed, err := selector.ParseSelector(selectorSpec)
	if err != nil {
		return mgs.toChans(fakeResponse{nil, []error{fmt.Errorf("Invalid selector")}, nil})
	}
	request := fakeRequest{root, parsed}
	mgs.receivedRequests = append(mgs.receivedRequests, request)
	for _, stub := range mgs.stubs {
		if reflect.DeepEqual(stub.request, request) {
			return mgs.toChans(stub.response)
		}
	}
	return mgs.toChans(fakeResponse{nil, []error{fmt.Errorf("Failed Request")}, nil})
}

func makeGsResponse(path string, blockCid cid.Cid) graphsync.ResponseProgress {
	return graphsync.ResponseProgress{
		Path: ipld.ParsePath(path),
		LastBlock: struct {
			Path ipld.Path
			Link ipld.Link
		}{
			Link: cidlink.Link{Cid: blockCid},
		},
	}
}
