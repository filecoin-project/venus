package net_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

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
	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/net"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestGraphsyncFetcher(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	bv := consensus.NewDefaultBlockValidator(5*time.Millisecond, clock.NewSystemClock())
	pid0 := th.RequireIntPeerID(t, 0)
	builder := chain.NewBuilder(t, address.Undef)
	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	rm := types.NewReceiptMaker()
	type notDecodable struct {
		num    int
		mesage string
	}
	cbor.RegisterCborType(notDecodable{})
	notDecodableBlock, err := cbor.WrapObject(notDecodable{5, "applesauce"}, types.DefaultHashFunction, -1)
	require.NoError(t, err)

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

	doneAt := func(tsKey types.TipSetKey) func(types.TipSet) (bool, error) {
		return func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(tsKey) {
				return true, nil
			}
			return false, nil
		}
	}
	withMessageBuilder := func(b *chain.BlockBuilder) {
		b.AddMessages(
			[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
			[]*types.MessageReceipt{rm.NewReceipt()},
		)
	}
	withMessageEachBuilder := func(b *chain.BlockBuilder, i int) {
		withMessageBuilder(b)
	}

	verifyMessagesAndReceiptsFetched := func(t *testing.T, ts types.TipSet) {
		for i := 0; i < ts.Len(); i++ {
			blk := ts.At(i)
			rawBlock, err := bs.Get(blk.Messages)
			require.NoError(t, err)
			messages, err := types.DecodeMessages(rawBlock.RawData())
			require.NoError(t, err)
			expectedMessages, err := builder.LoadMessages(ctx, blk.Messages)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(messages, expectedMessages))
			rawBlock, err = bs.Get(blk.MessageReceipts)
			require.NoError(t, err)
			receipts, err := types.DecodeReceipts(rawBlock.RawData())
			require.NoError(t, err)
			expectedReceipts, err := builder.LoadReceipts(ctx, blk.MessageReceipts)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(receipts, expectedReceipts))
		}
	}

	loader := successLoader(ctx, builder)
	t.Run("happy path returns correct tipsets", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		mgs := newMockableGraphsync(ctx, bs, t)
		mgs.stubResponseWithLoader(pid0, layer1Selector, loader, final.Key().ToSlice()...)
		mgs.stubResponseWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(4)
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("initial request fails on a block but fallback peer succeeds", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		mgs := newMockableGraphsync(ctx, bs, t)
		pid0Loader := errorOnCidsLoader(loader, final.At(1).Cid(), final.At(2).Cid())
		pid1Loader := errorOnCidsLoader(loader, final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, pid0Loader, final.Key().ToSlice()...)
		mgs.expectRequestToRespondWithLoader(pid1, layer1Selector, pid1Loader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, layer1Selector, loader, final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)

		done := doneAt(gen.Key())
		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(7)
		mgs.verifyExpectations()
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("initial request fails and no other peers succeed", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}
		mgs := newMockableGraphsync(ctx, bs, t)
		errorLoader := errorOnCidsLoader(loader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorLoader, final.Key().ToSlice()...)
		mgs.expectRequestToRespondWithLoader(pid1, layer1Selector, errorLoader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, layer1Selector, errorLoader, final.At(1).Cid(), final.At(2).Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)

		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(7)
		mgs.verifyExpectations()
		require.Errorf(t, err, "Failed fetching tipset: %s", final.Key().String())
		require.Nil(t, ts)
	})

	t.Run("blocks present but are missing messages", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		mgs := newMockableGraphsync(ctx, bs, t)
		errorOnMessagesLoader := errorOnCidsLoader(loader, final.At(1).Messages, final.At(2).Messages)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorOnMessagesLoader, final.Key().ToSlice()...)

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := doneAt(gen.Key())
		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(3)
		mgs.verifyExpectations()
		require.Errorf(t, err, "Failed fetching tipset: %s", final.Key().String())
		require.Nil(t, ts)
	})

	t.Run("blocks present but are missing message receips", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		mgs := newMockableGraphsync(ctx, bs, t)
		errorOnMessagesReceiptsLoader := errorOnCidsLoader(loader, final.At(1).MessageReceipts, final.At(2).MessageReceipts)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorOnMessagesReceiptsLoader, final.Key().ToSlice()...)

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := doneAt(gen.Key())
		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(3)
		mgs.verifyExpectations()
		require.Errorf(t, err, "Failed fetching tipset: %s", final.Key().String())
		require.Nil(t, ts)
	})

	t.Run("blocks missing/message receipts but recoves through fall back", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		mgs := newMockableGraphsync(ctx, bs, t)
		errorOnMessagesLoader := errorOnCidsLoader(loader, final.At(1).Messages, final.At(2).Messages)
		errorOnMessagesReceiptsLoader := errorOnCidsLoader(loader, final.At(1).MessageReceipts, final.At(2).MessageReceipts)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorOnMessagesLoader, final.Key().ToSlice()...)
		mgs.expectRequestToRespondWithLoader(pid1, layer1Selector, errorOnMessagesReceiptsLoader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, layer1Selector, loader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(8)
		mgs.verifyExpectations()
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("partial response fail during recursive fetch recovers at fail point", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildManyOn(5, gen, withMessageBuilder)
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

		mgs := newMockableGraphsync(ctx, bs, t)
		pid0Loader := errorOnCidsLoader(loader, blocks[3].Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, pid0Loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), pid0Loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), pid0Loader, blocks[0].Cid())
		mgs.expectRequestToRespondWithLoader(pid1, recursiveSelector(4), loader, blocks[2].Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)

		done := func(ts types.TipSet) (bool, error) {
			if ts.Key().Equals(gen.Key()) {
				return true, nil
			}
			return false, nil
		}

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(4)
		mgs.verifyExpectations()
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

	t.Run("missing single block in multi block tip during recursive fetch", func(t *testing.T) {
		gen := builder.NewGenesis()
		multi := builder.BuildOn(gen, 3, withMessageEachBuilder)
		penultimate := builder.BuildManyOn(3, multi, withMessageBuilder)
		final := builder.BuildOn(penultimate, 1, withMessageEachBuilder)
		mgs := newMockableGraphsync(ctx, bs, t)
		errorInMultiBlockLoader := errorOnCidsLoader(loader, multi.At(1).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), errorInMultiBlockLoader, penultimate.At(0).Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(3)
		mgs.verifyExpectations()
		require.Errorf(t, err, "Failed fetching tipset: %s", multi.Key().String())
		require.Nil(t, ts)
	})

	t.Run("missing single block in multi block tip during recursive fetch, recover through fallback", func(t *testing.T) {
		gen := builder.NewGenesis()
		multi := builder.BuildOn(gen, 3, withMessageEachBuilder)
		withMultiParent := builder.BuildOn(multi, 1, withMessageEachBuilder)
		penultimate := builder.BuildManyOn(2, withMultiParent, withMessageBuilder)
		final := builder.BuildOn(penultimate, 1, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain1 := types.NewChainInfo(pid1, final.Key(), height)
		chain2 := types.NewChainInfo(pid2, final.Key(), height)
		pt := &fakePeerTracker{[]*types.ChainInfo{chain1, chain2}}

		mgs := newMockableGraphsync(ctx, bs, t)
		errorInMultiBlockLoader := errorOnCidsLoader(loader, multi.At(1).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), errorInMultiBlockLoader, penultimate.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid1, recursiveSelector(4), loader, withMultiParent.At(0).Cid())

		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, pt)
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(4)
		mgs.verifyExpectations()
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

	t.Run("stopping at edge heights in recursive fetch", func(t *testing.T) {
		gen := builder.NewGenesis()
		recursive16stop := builder.BuildManyOn(1, gen, withMessageBuilder)
		recursive16middle := builder.BuildManyOn(15, recursive16stop, withMessageBuilder)
		recursive4stop := builder.BuildManyOn(1, recursive16middle, withMessageBuilder)
		recursive4middle := builder.BuildManyOn(3, recursive4stop, withMessageBuilder)
		recursive1stop := builder.BuildManyOn(1, recursive4middle, withMessageBuilder)
		final := builder.BuildOn(recursive1stop, 1, withMessageEachBuilder)

		t.Run("edge height 1", func(t *testing.T) {
			mgs := newMockableGraphsync(ctx, bs, t)
			mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
			mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
			fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})
			done := doneAt(recursive1stop.Key())

			ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
			require.NoError(t, err, "the request completes successfully")
			mgs.verifyReceivedRequestCount(2)
			mgs.verifyExpectations()

			require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
			lastTs := ts[len(ts)-1]
			verifyMessagesAndReceiptsFetched(t, lastTs)
		})

		t.Run("edge height 4", func(t *testing.T) {
			mgs := newMockableGraphsync(ctx, bs, t)
			mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
			mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
			mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), loader, recursive1stop.At(0).Cid())

			fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})
			done := doneAt(recursive4stop.Key())

			ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
			require.NoError(t, err, "the request completes successfully")
			mgs.verifyReceivedRequestCount(3)
			mgs.verifyExpectations()

			require.Equal(t, 6, len(ts), "the right number of tipsets is returned")
			lastTs := ts[len(ts)-1]
			verifyMessagesAndReceiptsFetched(t, lastTs)
		})

		t.Run("edge height 16", func(t *testing.T) {
			mgs := newMockableGraphsync(ctx, bs, t)
			mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
			mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
			mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), loader, recursive1stop.At(0).Cid())
			mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(16), loader, recursive4stop.At(0).Cid())

			fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})
			done := doneAt(recursive16stop.Key())

			ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
			require.NoError(t, err, "the request completes successfully")
			mgs.verifyReceivedRequestCount(4)
			mgs.verifyExpectations()

			require.Equal(t, 22, len(ts), "the right number of tipsets is returned")
			lastTs := ts[len(ts)-1]
			verifyMessagesAndReceiptsFetched(t, lastTs)
		})
	})

	t.Run("value returned with non block format", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, t)

		key := types.NewTipSetKey(notDecodableBlock.Cid())
		notDecodableLoader := simpleLoader([]format.Node{notDecodableBlock})
		mgs.stubResponseWithLoader(pid0, layer1Selector, notDecodableLoader, notDecodableBlock.Cid())
		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.Errorf(t, err, "fetched data (cid %s) was not a block", notDecodableBlock.Cid())
		require.Nil(t, ts)
	})

	t.Run("block returned with invalid syntax", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, t)
		block := simpleBlock()
		block.Height = 1
		key := types.NewTipSetKey(block.Cid())
		invalidSyntaxLoader := simpleLoader([]format.Node{block.ToNode()})
		mgs.stubResponseWithLoader(pid0, layer1Selector, invalidSyntaxLoader, block.Cid())
		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})
		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.Errorf(t, err, "block %s has nil StateRoot", block.Cid().String())
		require.Nil(t, ts)
	})

	t.Run("blocks present but messages don't decode", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, t)
		block := requireSimpleValidBlock(t, 3, address.Undef)
		block.Messages = notDecodableBlock.Cid()
		key := types.NewTipSetKey(block.Cid())
		notDecodableLoader := simpleLoader([]format.Node{block.ToNode(), notDecodableBlock, types.ReceiptCollection{}.ToNode()})
		mgs.stubResponseWithLoader(pid0, layer1Selector, notDecodableLoader, block.Cid())
		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.Errorf(t, err, "fetched data (cid %s) was not a message collection", notDecodableBlock.Cid())
		require.Nil(t, ts)
	})

	t.Run("blocks present but receipts don't decode", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, t)
		block := requireSimpleValidBlock(t, 3, address.Undef)
		block.MessageReceipts = notDecodableBlock.Cid()
		key := types.NewTipSetKey(block.Cid())
		notDecodableLoader := simpleLoader([]format.Node{block.ToNode(), notDecodableBlock, types.MessageCollection{}.ToNode()})
		mgs.stubResponseWithLoader(pid0, layer1Selector, notDecodableLoader, block.Cid())
		fetcher := net.NewGraphSyncFetcher(ctx, mgs, bs, bv, &fakePeerTracker{})

		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.Errorf(t, err, "fetched data (cid %s) was not a message receipt collection", notDecodableBlock.Cid())
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

// blockAndMessageProvider is any interface that can load blocks, messages, AND
// message receipts (such as a chain builder)
type blockAndMessageProvider interface {
	chain.MessageProvider
	chain.BlockProvider
}

func tryBlockMessageReceiptNode(ctx context.Context, f blockAndMessageProvider, c cid.Cid) (format.Node, error) {
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

// mockGraphsyncLoader is a function that loads cids into ipld.Nodes (or errors),
// used to construct a mock query result against a CID and a selector
type mockGraphsyncLoader func(cid.Cid) (format.Node, error)

// successLoader will load any cids returned by the given block and message provider
// or error otherwise
func successLoader(ctx context.Context, provider blockAndMessageProvider) mockGraphsyncLoader {
	return func(cidToLoad cid.Cid) (format.Node, error) {
		return tryBlockMessageReceiptNode(ctx, provider, cidToLoad)
	}
}

// errorOnCidsLoader will override a base loader to error for the specified cids
// or otherwise return the results from the base loader
func errorOnCidsLoader(baseLoader mockGraphsyncLoader, errorOnCids ...cid.Cid) mockGraphsyncLoader {
	return func(cidToLoad cid.Cid) (format.Node, error) {
		for _, testCid := range errorOnCids {
			if cidToLoad.Equals(testCid) {
				return nil, fmt.Errorf("Everything failed")
			}
		}
		return baseLoader(cidToLoad)
	}
}

// simple loader loads cids from a simple array of nodes
func simpleLoader(store []format.Node) mockGraphsyncLoader {
	cidsToNodes := make(map[cid.Cid]format.Node, len(store))
	for _, node := range store {
		cidsToNodes[node.Cid()] = node
	}
	return func(cidToLoad cid.Cid) (format.Node, error) {
		node, has := cidsToNodes[cidToLoad]
		if !has {
			return nil, fmt.Errorf("Everything failed")
		}
		return node, nil
	}
}

// fake request captures the parameters neccesary to uniquely
// identify a graphsync request
type fakeRequest struct {
	p        peer.ID
	root     ipld.Link
	selector selector.Selector
}

// fake response represents the neccesary data to simulate a graphsync query
// a graphsync query has:
// - two return values:
//   - a channel of ResponseProgress
//   - a channel of errors
// - one side effect:
//   - blocks written to a block store
// when graphsync is called for a matching request,
//   -- the responses array is converted to a channel
//   -- the error array is converted to a channel
//   -- a blks array is written to mock graphsync block store
type fakeResponse struct {
	responses []graphsync.ResponseProgress
	errs      []error
	blks      []format.Node
}

// request response just records a request and the respond to send when its
// made for a stub
type requestResponse struct {
	request  fakeRequest
	response fakeResponse
}

// mockableGraphsync conforms to the graphsync exchange interface needed by
// the graphsync fetcher but will only send stubbed responses
type mockableGraphsync struct {
	ctx              context.Context
	stubs            []requestResponse
	expectedRequests []fakeRequest
	receivedRequests []fakeRequest
	store            bstore.Blockstore
	t                *testing.T
}

func newMockableGraphsync(ctx context.Context, store bstore.Blockstore, t *testing.T) *mockableGraphsync {
	return &mockableGraphsync{
		ctx:   ctx,
		store: store,
		t:     t,
	}
}

// expect request will record a given set of requests as "expected", which can
// then be verified against received requests in verify expectations
func (mgs *mockableGraphsync) expectRequest(pid peer.ID, s selector.Selector, cids ...cid.Cid) {
	for _, c := range cids {
		mgs.expectedRequests = append(mgs.expectedRequests, fakeRequest{pid, cidlink.Link{Cid: c}, s})
	}
}

// verifyReceivedRequestCount will fail a test if the expected number of requests were not received
func (mgs *mockableGraphsync) verifyReceivedRequestCount(n int) {
	require.Equal(mgs.t, n, len(mgs.receivedRequests), "correct number of graphsync requests were made")
}

// verifyExpectations will fail a test if all expected requests were not received
func (mgs *mockableGraphsync) verifyExpectations() {
	for _, expectedRequest := range mgs.expectedRequests {
		matchedRequest := false
		for _, receivedRequest := range mgs.receivedRequests {
			if reflect.DeepEqual(expectedRequest, receivedRequest) {
				matchedRequest = true
				break
			}
		}
		require.True(mgs.t, matchedRequest, "expected request was made for peer %s, cid %s", expectedRequest.p.String(), expectedRequest.root.String())
	}
}

// stubResponseWithLoader stubs a response when the mocked graphsync
// instance is called with the given peer, selector, one of the cids
// by executing the specified root and selector using the given cid loader
func (mgs *mockableGraphsync) stubResponseWithLoader(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, cids ...cid.Cid) {
	for _, c := range cids {
		mgs.stubSingleResponseWithLoader(pid, s, loader, c)
	}
}

// stubResponseWithLoader stubs a response when the mocked graphsync
// instance is called with the given peer, selector, and cid
// by executing the specified root and selector using the given cid loader
func (mgs *mockableGraphsync) stubSingleResponseWithLoader(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, c cid.Cid) {
	var blks []format.Node
	var responses []graphsync.ResponseProgress

	linkLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		node, err := loader(cid)
		if err != nil {
			return nil, err
		}
		blks = append(blks, node)
		return bytes.NewBuffer(node.RawData()), nil
	}
	root := cidlink.Link{Cid: c}
	node, err := root.Load(mgs.ctx, ipld.LinkContext{}, ipldfree.NodeBuilder(), linkLoader)
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
			LinkLoader: linkLoader,
		},
	}.TraverseInformatively(node, s, visitor)
	mgs.stubs = append(mgs.stubs, requestResponse{
		fakeRequest{pid, root, s},
		fakeResponse{responses, []error{err}, blks},
	})
}

// expectRequestToRespondWithLoader is just a combination of an expectation and a stub --
// it expects the request to come in and responds with the given loader
func (mgs *mockableGraphsync) expectRequestToRespondWithLoader(pid peer.ID, s selector.Selector, loader mockGraphsyncLoader, cids ...cid.Cid) {
	mgs.expectRequest(pid, s, cids...)
	mgs.stubResponseWithLoader(pid, s, loader, cids...)
}

func (mgs *mockableGraphsync) processResponse(mr fakeResponse) (<-chan graphsync.ResponseProgress, <-chan error) {
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
		return mgs.processResponse(fakeResponse{nil, []error{fmt.Errorf("invalid selector")}, nil})
	}
	request := fakeRequest{p, root, parsed}
	mgs.receivedRequests = append(mgs.receivedRequests, request)
	for _, stub := range mgs.stubs {
		if reflect.DeepEqual(stub.request, request) {
			return mgs.processResponse(stub.response)
		}
	}
	return mgs.processResponse(fakeResponse{nil, []error{fmt.Errorf("unexpected request")}, nil})
}

type fakePeerTracker struct {
	peers []*types.ChainInfo
}

func (fpt *fakePeerTracker) List() []*types.ChainInfo {
	return fpt.peers
}

func simpleBlock() *types.Block {
	return &types.Block{
		ParentWeight:    0,
		Parents:         types.NewTipSetKey(),
		Height:          0,
		Messages:        types.EmptyMessagesCID,
		MessageReceipts: types.EmptyReceiptsCID,
	}
}

func requireSimpleValidBlock(t *testing.T, nonce uint64, miner address.Address) *types.Block {
	b := simpleBlock()
	ticket := make(types.Signature, binary.Size(nonce))
	binary.BigEndian.PutUint64(ticket, nonce)

	b.Ticket = ticket
	bytes, err := cbor.DumpObject("null")
	require.NoError(t, err)
	b.StateRoot, _ = cid.Prefix{
		Version:  1,
		Codec:    cid.DagCBOR,
		MhType:   types.DefaultHashFunction,
		MhLength: -1,
	}.Sum(bytes)
	b.Miner = miner
	return b
}
