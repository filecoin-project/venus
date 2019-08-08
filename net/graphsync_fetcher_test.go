package net_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dss "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-graphsync"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	ipld "github.com/ipld/go-ipld-prime"
	ipldfree "github.com/ipld/go-ipld-prime/impl/free"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	selectorbuilder "github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestGraphsyncFetcher(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	bv := th.NewFakeBlockValidator()
	pid0 := th.RequireIntPeerID(t, 0)
	builder := chain.NewBuilder(t, address.Undef)

	ssb := selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder())
	layer1Selector, err := ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", ssb.Matcher())
		efsb.Insert("messageReceipts", ssb.Matcher())
	}).Selector()
	require.NoError(t, err)
	gsSelector, err := ssb.ExploreRecursive(1, ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", ssb.Matcher())
		efsb.Insert("messageReceipts", ssb.Matcher())
		efsb.Insert("parents", ssb.ExploreUnion(
			ssb.ExploreAll(ssb.Matcher()),
			ssb.ExploreIndex(0, ssb.ExploreRecursiveEdge()),
		))
	})).Selector()
	require.NoError(t, err)
	gsSelectorRound2, err := ssb.ExploreRecursive(4, ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", ssb.Matcher())
		efsb.Insert("messageReceipts", ssb.Matcher())
		efsb.Insert("parents", ssb.ExploreUnion(
			ssb.ExploreAll(ssb.Matcher()),
			ssb.ExploreIndex(0, ssb.ExploreRecursiveEdge()),
		))
	})).Selector()
	require.NoError(t, err)
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)

	t.Run("happy path returns correct tipsets", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.AppendOn(gen, 3)

		stubs := []requestResponse{
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(0).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(0).ToNode()}},
			},
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(1).ToNode()}},
			},
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(2).ToNode()}},
			},
			{fakeRequest{pid0, cidlink.Link{Cid: final.At(0).Cid()}, gsSelector}, fakeResponse{
				responses: []graphsync.ResponseProgress{
					makeGsResponse("", final.At(0).Cid()),
					makeGsResponse("parents", final.At(0).Cid()),
					makeGsResponse("parents/0", gen.At(0).Cid()),
				},
				blks: []format.Node{final.At(0).ToNode(), gen.At(0).ToNode()},
			}},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(gen.Key()) {
				return true, nil
			}
			return false, nil
		}

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		require.Equal(t, 4, len(mgs.receivedRequests), "all expected graphsync requests are made")
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("initial request fails on a block but fallback peer succeeds", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.AppendOn(gen, 3)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		stubs := []requestResponse{
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(0).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(0).ToNode()}},
			},
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid1, cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(1).ToNode()}},
			},
			{
				fakeRequest{pid1, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid2, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(2).ToNode()}},
			},
			{fakeRequest{pid2, cidlink.Link{Cid: final.At(0).Cid()}, gsSelector}, fakeResponse{
				responses: []graphsync.ResponseProgress{
					makeGsResponse("", final.At(0).Cid()),
					makeGsResponse("parents", final.At(0).Cid()),
					makeGsResponse("parents/0", gen.At(0).Cid()),
				},
				blks: []format.Node{final.At(0).ToNode(), gen.At(0).ToNode()},
			}},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(gen.Key()) {
				return true, nil
			}
			return false, nil
		}

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		require.Equal(t, 7, len(mgs.receivedRequests), "all expected graphsync requests are made")
		require.Equal(t, pid0, mgs.receivedRequests[0].p, "asks first peer for everything in first tipset")
		require.Equal(t, pid0, mgs.receivedRequests[1].p, "asks first peer for everything in first tipset")
		require.Equal(t, pid0, mgs.receivedRequests[2].p, "asks first peer for everything in first tipset")
		require.Equal(t, pid1, mgs.receivedRequests[3].p, "asks second peer for failed responses in first tipset")
		require.Equal(t, pid1, mgs.receivedRequests[4].p, "asks second peer for failed responses in first tipset")
		require.Equal(t, pid2, mgs.receivedRequests[5].p, "asks third peer for failed responses in first tipset")
		require.Equal(t, pid2, mgs.receivedRequests[6].p, "asks third peer for remaining tipsets")
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("initial request fails and no other peers succeed", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.AppendOn(gen, 3)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		stubs := []requestResponse{
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(0).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(0).ToNode()}},
			},
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid1, cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid1, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid2, cidlink.Link{Cid: final.At(1).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
			{
				fakeRequest{pid2, cidlink.Link{Cid: final.At(2).Cid()}, layer1Selector},
				fakeResponse{nil, []error{fmt.Errorf("Everything failed")}, nil},
			},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(gen.Key()) {
				return true, nil
			}
			return false, nil
		}

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.Equal(t, 7, len(mgs.receivedRequests), "all expected graphsync requests are made")
		require.Equal(t, pid0, mgs.receivedRequests[0].p)
		require.Equal(t, pid0, mgs.receivedRequests[1].p)
		require.Equal(t, pid0, mgs.receivedRequests[2].p)
		require.Equal(t, pid1, mgs.receivedRequests[3].p)
		require.Equal(t, pid1, mgs.receivedRequests[4].p)
		require.Equal(t, pid2, mgs.receivedRequests[5].p)
		require.Equal(t, pid2, mgs.receivedRequests[6].p)
		require.Errorf(t, err, "Failed fetching tipset: %s", final.Key().String())
		require.Nil(t, ts)
	})

	t.Run("partial response fail during recursive fetch recovers at fail point", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.AppendManyOn(5, gen)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		middleNodes := make([]format.Node, 4)
		current := final
		for i := 0; i < 4; i++ {
			key, err := current.Parents()
			require.NoError(t, err)
			current, err = builder.GetTipSet(key)
			require.NoError(t, err)
			middleNodes[i] = current.At(0).ToNode()
		}

		stubs := []requestResponse{
			{
				fakeRequest{pid0, cidlink.Link{Cid: final.At(0).Cid()}, layer1Selector},
				fakeResponse{blks: []format.Node{final.At(0).ToNode()}},
			},
			{fakeRequest{pid0, cidlink.Link{Cid: final.At(0).Cid()}, gsSelector}, fakeResponse{
				responses: []graphsync.ResponseProgress{
					makeGsResponse("", final.At(0).Cid()),
					makeGsResponse("parents", final.At(0).Cid()),
					makeGsResponse("parents/0", middleNodes[0].Cid()),
				},
				blks: []format.Node{final.At(0).ToNode(), middleNodes[0]},
			}},
			{fakeRequest{pid0, cidlink.Link{Cid: middleNodes[0].Cid()}, gsSelectorRound2}, fakeResponse{
				responses: []graphsync.ResponseProgress{
					makeGsResponse("", middleNodes[0].Cid()),
					makeGsResponse("parents", middleNodes[0].Cid()),
					makeGsResponse("parents/0", middleNodes[1].Cid()),
					makeGsResponse("parents/0/parents", middleNodes[1].Cid()),
					makeGsResponse("parents/0/parents/0", middleNodes[2].Cid()),
				},
				errs: []error{fmt.Errorf("Everything failed")},
				blks: []format.Node{middleNodes[0], middleNodes[1], middleNodes[2]},
			}},
			{fakeRequest{pid1, cidlink.Link{Cid: middleNodes[2].Cid()}, gsSelectorRound2}, fakeResponse{
				responses: []graphsync.ResponseProgress{
					makeGsResponse("", middleNodes[2].Cid()),
					makeGsResponse("parents", middleNodes[2].Cid()),
					makeGsResponse("parents/0", middleNodes[3].Cid()),
					makeGsResponse("parents/0/parents", middleNodes[3].Cid()),
					makeGsResponse("parents/0/parents/0", gen.At(0).Cid()),
					makeGsResponse("parents/0/parents/0/parents", gen.At(0).Cid()),
				},
				blks: []format.Node{middleNodes[2], middleNodes[3], gen.At(0).ToNode()},
			}},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(gen.Key()) {
				return true, nil
			}
			return false, nil
		}

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		require.Equal(t, 4, len(mgs.receivedRequests), "all expected graphsync requests are made")
		require.Equal(t, pid0, mgs.receivedRequests[0].p)
		require.Equal(t, pid0, mgs.receivedRequests[1].p)
		require.Equal(t, pid0, mgs.receivedRequests[2].p)
		require.Equal(t, pid1, mgs.receivedRequests[3].p)
		require.Equal(t, 6, len(ts), "the right number of tipsets is returned")
		expectedTs := final
		for _, resultTs := range ts {
			require.True(t, expectedTs.Key().Equals(resultTs.Key()), "the initial tipset is correct")
			key, err := expectedTs.Parents()
			require.NoError(t, err)
			if !key.Empty() {
				expectedTs, err = builder.GetTipSet(key)
				require.NoError(t, err)
			}
		}
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
				fakeRequest{pid0, cidlink.Link{Cid: notABlockCid}, layer1Selector},
				fakeResponse{blks: []format.Node{notABlockObj}},
			},
		}
		mgs := &mockableGraphsync{stubs: stubs, t: t, store: bs}

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(originalCids) {
				return true, nil
			}
			return false, nil
		}
		ts, err := fetcher.FetchTipSets(ctx, originalCids, pid0, done)
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
	antepenultimate := builder.AppendManyOn(29, gen)

	// add a message and receipt to the head's parent
	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	msg1 := mm.NewSignedMessage(mm.Addresses()[0], 1)
	rcpt1 := &types.MessageReceipt{ExitCode: 42}
	penultimate := builder.BuildOn(antepenultimate, func(bb *chain.BlockBuilder) {
		bb.AddMessages([]*types.SignedMessage{msg1}, []*types.MessageReceipt{rcpt1})
	})

	// add a message and receipt to the head
	msg2 := mm.NewSignedMessage(mm.Addresses()[0], 2)
	rcpt2 := &types.MessageReceipt{ExitCode: 67}
	final := builder.BuildOn(penultimate, func(bb *chain.BlockBuilder) {
		bb.AddMessages([]*types.SignedMessage{msg2}, []*types.MessageReceipt{rcpt2})
	})

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
	pt := net.NewPeerTracker()

	localLoader := gsstoreutil.LoaderForBlockstore(bs)
	localStorer := gsstoreutil.StorerForBlockstore(bs)

	localGraphsync := graphsync.New(ctx, gsnet1, bridge1, localLoader, localStorer)

	fetcher := net.NewGraphSyncFetcher(ctx, localGraphsync, bs, bv, pt)

	remoteLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		raw, err := tryBlockMessageReceipt(ctx, builder, cid)
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

	require.Equal(t, 32, len(tipsets))

	for _, ts := range tipsets {
		matchedTs, err := builder.GetTipSet(ts.Key())
		require.NoError(t, err)
		require.NotNil(t, matchedTs)
	}

	// check that the fetcher's storage has messages and receipts linked by
	// the chain.
	hasMsg1, err := bs.Has(types.MessageCollection([]*types.SignedMessage{msg1}).Cid())
	require.NoError(t, err)
	assert.True(t, hasMsg1)
	hasRcpt1, err := bs.Has(types.ReceiptCollection([]*types.MessageReceipt{rcpt1}).Cid())
	require.NoError(t, err)
	assert.True(t, hasRcpt1)
	hasMsg2, err := bs.Has(types.MessageCollection([]*types.SignedMessage{msg2}).Cid())
	require.NoError(t, err)
	assert.True(t, hasMsg2)
	hasRcpt2, err := bs.Has(types.ReceiptCollection([]*types.MessageReceipt{rcpt2}).Cid())
	require.NoError(t, err)
	assert.True(t, hasRcpt2)
}

func tryBlockMessageReceipt(ctx context.Context, f *chain.Builder, c cid.Cid) ([]byte, error) {
	if block, err := f.GetBlock(ctx, c); err == nil {
		return cbor.DumpObject(block)
	}
	if messages, err := f.LoadMessages(ctx, c); err == nil {
		return cbor.DumpObject(messages)
	}
	if receipts, err := f.LoadReceipts(ctx, c); err == nil {
		return cbor.DumpObject(receipts)
	}
	return nil, fmt.Errorf("cid could not be resolved through builder")
}

type fakeRequest struct {
	p        peer.ID
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
	for _, block := range mr.blks {
		requireBlockStorePut(mgs.t, mgs.store, block)
	}

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

	return responseChan, errChan
}

func (mgs *mockableGraphsync) Request(ctx context.Context, p peer.ID, root ipld.Link, selectorSpec ipld.Node) (<-chan graphsync.ResponseProgress, <-chan error) {
	parsed, err := selector.ParseSelector(selectorSpec)
	if err != nil {
		return mgs.toChans(fakeResponse{nil, []error{fmt.Errorf("Invalid selector")}, nil})
	}
	request := fakeRequest{p, root, parsed}
	mgs.receivedRequests = append(mgs.receivedRequests, request)
	for _, stub := range mgs.stubs {
		if reflect.DeepEqual(stub.request, request) {
			return mgs.toChans(stub.response)
		}
	}
	return mgs.toChans(fakeResponse{nil, []error{fmt.Errorf("Missing Mocked Request")}, nil})
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

type fakePeerTracker struct {
	peers []*types.ChainInfo
}

func (fpt *fakePeerTracker) List() []*types.ChainInfo {
	return fpt.peers
}
