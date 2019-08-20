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
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-ipld-prime"
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
	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]

	ssb := selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder())
	layer1Selector, err := ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", ssb.Matcher())
		efsb.Insert("messageReceipts", ssb.Matcher())
	}).Selector()
	require.NoError(t, err)
	recursiveSelector := func(levels int) selector.Selector {
		s, err := ssb.ExploreRecursive(levels, ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
			efsb.Insert("parents", ssb.ExploreUnion(
				ssb.ExploreAll(
					ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
						efsb.Insert("messages", ssb.Matcher())
						efsb.Insert("messageReceipts", ssb.Matcher())
					}),
				),
				ssb.ExploreIndex(0, ssb.ExploreRecursiveEdge()),
			))
		})).Selector()
		require.NoError(t, err)
		return s
	}
	pid1 := th.RequireIntPeerID(t, 1)
	pid2 := th.RequireIntPeerID(t, 2)

	t.Run("happy path returns correct tipsets", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, func(b *chain.BlockBuilder, i int) {
			b.AddMessages(
				[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
				types.EmptyReceipts(1),
			)
		})
		mgs := newMockableGraphsync(ctx, builder, bs, t)
		mgs.stubResponse(pid0, final.At(0).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid0, final.At(1).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid0, final.At(2).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid0, final.At(0).Cid(), recursiveSelector(1), nil)

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
		final := builder.BuildOn(gen, 3, func(b *chain.BlockBuilder, i int) {
			b.AddMessages(
				[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
				types.EmptyReceipts(1),
			)
		})
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		mgs := newMockableGraphsync(ctx, builder, bs, t)
		mgs.stubResponse(pid0, final.At(0).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid0, final.At(1).Cid(), layer1Selector, []cid.Cid{final.At(1).Cid()})
		mgs.stubResponse(pid0, final.At(2).Cid(), layer1Selector, []cid.Cid{final.At(2).Cid()})
		mgs.stubResponse(pid1, final.At(1).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid1, final.At(2).Cid(), layer1Selector, []cid.Cid{final.At(2).Cid()})
		mgs.stubResponse(pid2, final.At(2).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid2, final.At(2).Cid(), recursiveSelector(1), nil)

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
		final := builder.BuildOn(gen, 3, func(b *chain.BlockBuilder, i int) {
			b.AddMessages(
				[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
				types.EmptyReceipts(1),
			)
		})
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}
		mgs := newMockableGraphsync(ctx, builder, bs, t)
		mgs.stubResponse(pid0, final.At(0).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid0, final.At(1).Cid(), layer1Selector, []cid.Cid{final.At(1).Cid()})
		mgs.stubResponse(pid0, final.At(2).Cid(), layer1Selector, []cid.Cid{final.At(2).Cid()})
		mgs.stubResponse(pid1, final.At(1).Cid(), layer1Selector, []cid.Cid{final.At(1).Cid()})
		mgs.stubResponse(pid1, final.At(2).Cid(), layer1Selector, []cid.Cid{final.At(2).Cid()})
		mgs.stubResponse(pid2, final.At(1).Cid(), layer1Selector, []cid.Cid{final.At(1).Cid()})
		mgs.stubResponse(pid2, final.At(2).Cid(), layer1Selector, []cid.Cid{final.At(2).Cid()})

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
		final := builder.BuildManyOn(5, gen, func(b *chain.BlockBuilder) {
			b.AddMessages(
				[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
				types.EmptyReceipts(1),
			)
		})
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		blocks := make([]*types.Block, 4) // in fetch order
		prev := final.At(0)
		for i := 0; i < 4; i++ {
			parent := prev.Parents.Iter().Value()
			prev, err = builder.GetBlock(ctx, parent)
			require.NoError(t, err)
			blocks[i] = prev
		}

		mgs := newMockableGraphsync(ctx, builder, bs, t)
		mgs.stubResponse(pid0, final.At(0).Cid(), layer1Selector, nil)
		mgs.stubResponse(pid0, final.At(0).Cid(), recursiveSelector(1), nil)
		mgs.stubResponse(pid0, blocks[0].Cid(), recursiveSelector(4), []cid.Cid{blocks[3].Cid()})
		mgs.stubResponse(pid1, blocks[2].Cid(), recursiveSelector(4), nil)

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
	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]
	gen := builder.NewGenesis()
	i := uint64(0)
	tipCount := 32
	final := builder.BuildManyOn(tipCount, gen, func(b *chain.BlockBuilder) {
		b.AddMessages(
			[]*types.SignedMessage{mm.NewSignedMessage(alice, i)},
			[]*types.MessageReceipt{{ExitCode: uint8(i)}},
		)
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
		node, err := tryBlockMessageReceiptNode(ctx, builder, cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(node.RawData()), nil
	}
	graphsync.New(ctx, gsnet2, bridge2, remoteLoader, nil)

	tipsets, err := fetcher.FetchTipSets(ctx, final.Key(), host2.ID(), func(ts types.TipSet) (bool, error) {
		if ts.Key().Equals(gen.Key()) {
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)

	require.Equal(t, tipCount+1, len(tipsets))

	// Check the headers, messages, and receipt structures are in the store.
	expectedTips := builder.RequireTipSets(final.Key(), tipCount+1)
	for _, ts := range expectedTips {
		stored, err := bs.Has(ts.At(0).Cid())
		require.NoError(t, err)
		assert.True(t, stored)

		stored, err = bs.Has(ts.At(0).Messages)
		require.NoError(t, err)
		assert.True(t, stored)

		stored, err = bs.Has(ts.At(0).MessageReceipts)
		require.NoError(t, err)
		assert.True(t, stored)
	}
}

func tryBlockMessageReceiptNode(ctx context.Context, f *chain.Builder, c cid.Cid) (format.Node, error) {
	if block, err := f.GetBlock(ctx, c); err == nil {
		return block.ToNode(), nil
	}
	if messages, err := f.LoadMessages(ctx, c); err == nil {
		return types.MessageCollection(messages).ToNode(), nil
	}
	if receipts, err := f.LoadReceipts(ctx, c); err == nil {
		return types.ReceiptCollection(receipts).ToNode(), nil
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
	ctx              context.Context
	builder          *chain.Builder
	stubs            []requestResponse
	receivedRequests []fakeRequest
	store            bstore.Blockstore
	t                *testing.T
}

func newMockableGraphsync(ctx context.Context, builder *chain.Builder, store bstore.Blockstore, t *testing.T) *mockableGraphsync {
	return &mockableGraphsync{
		ctx:     ctx,
		builder: builder,
		store:   store,
		t:       t,
	}
}

func (mgs *mockableGraphsync) stubResponse(pid peer.ID, c cid.Cid, s selector.Selector, missingCids []cid.Cid) {
	var blks []format.Node
	var responses []graphsync.ResponseProgress

	loader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		for _, testCid := range missingCids {
			if cid.Equals(testCid) {
				return nil, fmt.Errorf("everything failed")
			}
		}
		node, err := tryBlockMessageReceiptNode(mgs.ctx, mgs.builder, cid)
		if err != nil {
			return nil, err
		}
		blks = append(blks, node)
		return bytes.NewBuffer(node.RawData()), nil
	}
	root := cidlink.Link{Cid: c}
	node, err := root.Load(mgs.ctx, ipld.LinkContext{}, ipldfree.NodeBuilder(), loader)
	if err != nil {
		mgs.stubs = append(mgs.stubs, requestResponse{
			fakeRequest{pid, root, s},
			fakeResponse{errs: []error{err}},
		})
		return
	}
	visitor := func(tp ipldbridge.TraversalProgress, n ipld.Node, tr ipldbridge.TraversalReason) error {
		responses = append(responses, graphsync.ResponseProgress{Node: n, Path: tp.Path, LastBlock: tp.LastBlock})
		return nil
	}
	err = ipldbridge.TraversalProgress{
		Cfg: &ipldbridge.TraversalConfig{
			Ctx:        mgs.ctx,
			LinkLoader: loader,
		},
	}.TraverseInformatively(node, s, visitor)
	mgs.stubs = append(mgs.stubs, requestResponse{
		fakeRequest{pid, root, s},
		fakeResponse{responses, []error{err}, blks},
	})
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
		return mgs.toChans(fakeResponse{nil, []error{fmt.Errorf("invalid selector")}, nil})
	}
	request := fakeRequest{p, root, parsed}
	mgs.receivedRequests = append(mgs.receivedRequests, request)
	for _, stub := range mgs.stubs {
		if reflect.DeepEqual(stub.request, request) {
			return mgs.toChans(stub.response)
		}
	}
	return mgs.toChans(fakeResponse{nil, []error{fmt.Errorf("unexpected request")}, nil})
}

type fakePeerTracker struct {
	peers []*types.ChainInfo
}

func (fpt *fakePeerTracker) List() []*types.ChainInfo {
	return fpt.peers
}
