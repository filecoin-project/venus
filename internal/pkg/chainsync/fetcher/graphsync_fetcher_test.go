package fetcher_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

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

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/fetcher"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	th "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

const visitsPerBlock = 18

type notDecodable struct {
	Num    int    `json:"num"`
	Mesage string `json:"mesage"`
}

func init() {
	encoding.RegisterIpldCborType(notDecodable{})
}

func TestGraphsyncFetcher(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	fc := th.NewFakeClock(time.Now())
	genTime := uint64(1234567890)
	blockTime := 5 * time.Second
	chainClock := clock.NewChainClockFromClock(genTime, blockTime, fc)
	bv := consensus.NewDefaultBlockValidator(chainClock)
	pid0 := th.RequireIntPeerID(t, 0)
	builder := chain.NewBuilderWithDeps(t, address.Undef, &chain.FakeStateBuilder{}, chain.NewClockTimestamper(chainClock))
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)
	notDecodableBlock, err := cbor.WrapObject(notDecodable{5, "applesauce"}, types.DefaultHashFunction, -1)
	require.NoError(t, err)

	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]

	ssb := selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder())

	amtSelector := ssb.ExploreIndex(2,
		ssb.ExploreRecursive(selector.RecursionLimitDepth(10),
			ssb.ExploreUnion(
				ssb.ExploreIndex(1, ssb.ExploreAll(ssb.ExploreRecursiveEdge())),
				ssb.ExploreIndex(2, ssb.ExploreAll(ssb.Matcher())))))

	layer1Selector, err := ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
		efsb.Insert("messages", ssb.ExploreFields(func(messagesSelector selectorbuilder.ExploreFieldsSpecBuilder) {
			messagesSelector.Insert("secpRoot", amtSelector)
			messagesSelector.Insert("bLSRoot", amtSelector)
		}))
	}).Selector()
	require.NoError(t, err)
	recursiveSelector := func(levels int) selector.Selector {
		s, err := ssb.ExploreRecursive(selector.RecursionLimitDepth(levels), ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
			efsb.Insert("parents", ssb.ExploreUnion(
				ssb.ExploreAll(
					ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
						efsb.Insert("messages", ssb.ExploreFields(func(messagesSelector selectorbuilder.ExploreFieldsSpecBuilder) {
							messagesSelector.Insert("secpRoot", amtSelector)
							messagesSelector.Insert("bLSRoot", amtSelector)
						}))
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

	doneAt := func(tsKey block.TipSetKey) func(block.TipSet) (bool, error) {
		return func(ts block.TipSet) (bool, error) {
			if ts.Key().Equals(tsKey) {
				return true, nil
			}
			return false, nil
		}
	}
	withMessageBuilder := func(b *chain.BlockBuilder) {
		b.AddMessages(
			[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
			[]*types.UnsignedMessage{&mm.NewSignedMessage(bob, 1).Message},
		)
	}
	withMessageEachBuilder := func(b *chain.BlockBuilder, i int) {
		withMessageBuilder(b)
	}

	verifyMessagesFetched := func(t *testing.T, ts block.TipSet) {
		for i := 0; i < ts.Len(); i++ {
			blk := ts.At(i)

			// use fetcher blockstore to retrieve messages
			msgStore := chain.NewMessageStore(bs)
			secpMsgs, blsMsgs, err := msgStore.LoadMessages(ctx, blk.Messages)
			require.NoError(t, err)

			// get expected messages from builders block store
			expectedSecpMessages, expectedBLSMsgs, err := builder.LoadMessages(ctx, blk.Messages)
			require.NoError(t, err)

			require.True(t, reflect.DeepEqual(secpMsgs, expectedSecpMessages))
			require.True(t, reflect.DeepEqual(blsMsgs, expectedBLSMsgs))
		}
	}

	loader := successLoader(ctx, builder)
	t.Run("happy path returns correct tipsets", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.stubResponseWithLoader(pid0, layer1Selector, loader, final.Key().ToSlice()...)
		mgs.stubResponseWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))
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
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)
		pt := newFakePeerTracker(chain0, chain1, chain2)

		mgs := newMockableGraphsync(ctx, bs, fc, t)
		pid0Loader := errorOnCidsLoader(loader, final.At(1).Cid(), final.At(2).Cid())
		pid1Loader := errorOnCidsLoader(loader, final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, pid0Loader, final.Key().ToSlice()...)
		mgs.expectRequestToRespondWithLoader(pid1, layer1Selector, pid1Loader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, layer1Selector, loader, final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, pt)

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
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)
		pt := newFakePeerTracker(chain0, chain1, chain2)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		errorLoader := errorOnCidsLoader(loader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorLoader, final.Key().ToSlice()...)
		mgs.expectRequestToRespondWithLoader(pid1, layer1Selector, errorLoader, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, layer1Selector, errorLoader, final.At(1).Cid(), final.At(2).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, pt)

		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(7)
		mgs.verifyExpectations()
		require.EqualError(t, err, fmt.Sprintf("fetching tipset: %s: Unable to find any untried peers", final.Key().String()))
		require.Nil(t, ts)
	})

	t.Run("requests fails because blocks are present but are missing messages", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		errorOnMessagesLoader := errorOnCidsLoader(loader, final.At(2).Messages.SecpRoot)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorOnMessagesLoader, final.Key().ToSlice()...)

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))

		done := doneAt(gen.Key())
		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(3)
		mgs.verifyExpectations()
		require.EqualError(t, err, fmt.Sprintf("fetching tipset: %s: Unable to find any untried peers", final.Key().String()))
		require.Nil(t, ts)
	})
	t.Run("partial response fail during recursive fetch recovers at fail point", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildManyOn(5, gen, withMessageBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)
		pt := newFakePeerTracker(chain0, chain1, chain2)

		blocks := make([]*block.Block, 4) // in fetch order
		prev := final.At(0)
		for i := 0; i < 4; i++ {
			parent := prev.Parents.Iter().Value()
			prev, err = builder.GetBlock(ctx, parent)
			require.NoError(t, err)
			blocks[i] = prev
		}

		mgs := newMockableGraphsync(ctx, bs, fc, t)
		pid0Loader := errorOnCidsLoader(loader, blocks[3].Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, pid0Loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), pid0Loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), pid0Loader, blocks[0].Cid())
		mgs.expectRequestToRespondWithLoader(pid1, recursiveSelector(4), loader, blocks[2].Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, pt)

		done := func(ts block.TipSet) (bool, error) {
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
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		errorInMultiBlockLoader := errorOnCidsLoader(loader, multi.At(1).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), errorInMultiBlockLoader, penultimate.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		mgs.verifyReceivedRequestCount(3)
		mgs.verifyExpectations()
		require.EqualError(t, err, fmt.Sprintf("fetching tipset: %s: Unable to find any untried peers", multi.Key().String()))
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
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)

		mgs := newMockableGraphsync(ctx, bs, fc, t)
		errorInMultiBlockLoader := errorOnCidsLoader(loader, multi.At(1).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), errorInMultiBlockLoader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), errorInMultiBlockLoader, penultimate.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid1, recursiveSelector(4), loader, withMultiParent.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0, chain1, chain2))
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
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		nextKey := final.Key()
		for i := 1; i <= 22; i++ {
			tipset, err := builder.GetTipSet(nextKey)
			require.NoError(t, err)
			mgs := newMockableGraphsync(ctx, bs, fc, t)
			mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
			receivedRequestCount := 1
			if i > 1 {
				mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
				receivedRequestCount++
			}
			if i > 2 {
				mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(4), loader, recursive1stop.At(0).Cid())
				receivedRequestCount++
			}
			if i > 6 {
				mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(16), loader, recursive4stop.At(0).Cid())
				receivedRequestCount++
			}

			fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))
			done := doneAt(tipset.Key())

			ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
			require.NoError(t, err, "the request completes successfully")
			mgs.verifyReceivedRequestCount(receivedRequestCount)
			mgs.verifyExpectations()

			require.Equal(t, i, len(ts), "the right number of tipsets is returned")
			lastTs := ts[len(ts)-1]
			verifyMessagesFetched(t, lastTs)

			nextKey, err = tipset.Parents()
			require.NoError(t, err)
		}
	})

	t.Run("value returned with non block format", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, fc, t)

		key := block.NewTipSetKey(notDecodableBlock.Cid())
		chain0 := block.NewChainInfo(pid0, pid0, key, 0)
		notDecodableLoader := simpleLoader([]format.Node{notDecodableBlock})
		mgs.stubResponseWithLoader(pid0, layer1Selector, notDecodableLoader, notDecodableBlock.Cid())
		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))

		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.EqualError(t, err, fmt.Sprintf("fetched data (cid %s) was not a block: unmarshal error: stream contains key \"num\", but there's no such field in structs of type block.Block", notDecodableBlock.Cid().String()))
		require.Nil(t, ts)
	})

	t.Run("block returned with invalid syntax", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		blk := simpleBlock()
		blk.Height = 1
		blk.Timestamp = types.Uint64(chainClock.StartTimeOfEpoch(types.NewBlockHeight(uint64(blk.Height))).Unix())
		key := block.NewTipSetKey(blk.Cid())
		chain0 := block.NewChainInfo(pid0, pid0, key, uint64(blk.Height))
		invalidSyntaxLoader := simpleLoader([]format.Node{blk.ToNode()})
		mgs.stubResponseWithLoader(pid0, layer1Selector, invalidSyntaxLoader, blk.Cid())
		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))
		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.EqualError(t, err, fmt.Sprintf("invalid block %s: block %s has nil StateRoot", blk.Cid().String(), blk.Cid().String()))
		require.Nil(t, ts)
	})

	t.Run("blocks present but messages don't decode", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		blk := requireSimpleValidBlock(t, 3, address.Undef)
		blk.Messages = types.TxMeta{SecpRoot: notDecodableBlock.Cid(), BLSRoot: types.EmptyMessagesCID}
		key := block.NewTipSetKey(blk.Cid())
		chain0 := block.NewChainInfo(pid0, pid0, key, uint64(blk.Height))
		nd, err := (&types.SignedMessage{}).ToNode()
		require.NoError(t, err)
		notDecodableLoader := simpleLoader([]format.Node{blk.ToNode(), notDecodableBlock, nd})
		mgs.stubResponseWithLoader(pid0, layer1Selector, notDecodableLoader, blk.Cid())
		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))

		done := doneAt(key)
		ts, err := fetcher.FetchTipSets(ctx, key, pid0, done)
		require.EqualError(t, err, fmt.Sprintf("fetched data (cid %s) could not be decoded as an AMT: cbor input should be of type array", notDecodableBlock.Cid().String()))
		require.Nil(t, ts)
	})

	t.Run("messages don't validate", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 1, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.stubResponseWithLoader(pid0, layer1Selector, loader, final.Key().ToSlice()...)

		errorMv := mockSyntaxValidator{
			validateMessagesError: fmt.Errorf("Everything Failed"),
		}
		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, errorMv, fc, newFakePeerTracker(chain0))
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.Nil(t, ts)
		require.Error(t, err, "invalid messages for for message collection (cid %s)", final.At(0).Messages.String())
	})

	t.Run("hangup occurs during first layer fetch but recovers through fallback", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)
		pt := newFakePeerTracker(chain0, chain1, chain2)

		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid0, layer1Selector, loader, 0, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid1, layer1Selector, loader, final.At(1).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid1, layer1Selector, loader, 0, final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, layer1Selector, loader, final.At(2).Cid())
		mgs.expectRequestToRespondWithLoader(pid2, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, pt)
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(7)
		mgs.verifyExpectations()
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
	})

	t.Run("initial request hangs up and no other peers succeed", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)
		pt := newFakePeerTracker(chain0, chain1, chain2)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid0, layer1Selector, loader, 0, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid1, layer1Selector, loader, 0, final.At(1).Cid(), final.At(2).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid2, layer1Selector, loader, 0, final.At(1).Cid(), final.At(2).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, pt)
		done := doneAt(gen.Key())
		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)

		mgs.verifyReceivedRequestCount(7)
		mgs.verifyExpectations()
		require.EqualError(t, err, fmt.Sprintf("fetching tipset: %s: Unable to find any untried peers", final.Key().String()))
		require.Nil(t, ts)
	})

	t.Run("partial response hangs up during recursive fetch recovers at hang up point", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildManyOn(5, gen, withMessageBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)
		pt := newFakePeerTracker(chain0, chain1, chain2)

		blocks := make([]*block.Block, 4) // in fetch order
		prev := final.At(0)
		for i := 0; i < 4; i++ {
			parent := prev.Parents.Iter().Value()
			prev, err = builder.GetBlock(ctx, parent)
			require.NoError(t, err)
			blocks[i] = prev
		}

		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid0, recursiveSelector(4), loader, 2*visitsPerBlock, blocks[0].Cid())
		mgs.expectRequestToRespondWithLoader(pid1, recursiveSelector(4), loader, blocks[2].Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, pt)

		done := func(ts block.TipSet) (bool, error) {
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

	t.Run("hangs up on single block in multi block tip during recursive fetch", func(t *testing.T) {
		gen := builder.NewGenesis()
		multi := builder.BuildOn(gen, 3, withMessageEachBuilder)
		penultimate := builder.BuildManyOn(3, multi, withMessageBuilder)
		final := builder.BuildOn(penultimate, 1, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid0, recursiveSelector(4), loader, 2*visitsPerBlock, penultimate.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSets(ctx, final.Key(), pid0, done)

		mgs.verifyReceivedRequestCount(3)
		mgs.verifyExpectations()
		require.EqualError(t, err, fmt.Sprintf("fetching tipset: %s: Unable to find any untried peers", multi.Key().String()))
		require.Nil(t, ts)
	})

	t.Run("hangs up on single block in multi block tip during recursive fetch, recover through fallback", func(t *testing.T) {
		gen := builder.NewGenesis()
		multi := builder.BuildOn(gen, 3, withMessageEachBuilder)
		withMultiParent := builder.BuildOn(multi, 1, withMessageEachBuilder)
		penultimate := builder.BuildManyOn(2, withMultiParent, withMessageBuilder)
		final := builder.BuildOn(penultimate, 1, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		chain1 := block.NewChainInfo(pid1, pid1, final.Key(), height)
		chain2 := block.NewChainInfo(pid2, pid2, final.Key(), height)

		mgs := newMockableGraphsync(ctx, bs, fc, t)
		mgs.expectRequestToRespondWithLoader(pid0, layer1Selector, loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())
		mgs.expectRequestToRespondWithHangupAfter(pid0, recursiveSelector(4), loader, 2*visitsPerBlock, penultimate.At(0).Cid())
		mgs.expectRequestToRespondWithLoader(pid1, recursiveSelector(4), loader, withMultiParent.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0, chain1, chain2))
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
}

func TestHeadersOnlyGraphsyncFetch(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	bs := bstore.NewBlockstore(dss.MutexWrap(datastore.NewMapDatastore()))
	fc := th.NewFakeClock(time.Now())
	genTime := uint64(1234567890)
	chainClock := clock.NewChainClockFromClock(genTime, 5*time.Second, fc)
	bv := consensus.NewDefaultBlockValidator(chainClock)
	pid0 := th.RequireIntPeerID(t, 0)
	builder := chain.NewBuilderWithDeps(t, address.Undef, &chain.FakeStateBuilder{}, chain.NewClockTimestamper(chainClock))
	keys := types.MustGenerateKeyInfo(1, 42)
	mm := types.NewMessageMaker(t, keys)
	notDecodableBlock, err := cbor.WrapObject(notDecodable{5, "applebutter"}, types.DefaultHashFunction, -1)
	require.NoError(t, err)

	alice := mm.Addresses()[0]

	ssb := selectorbuilder.NewSelectorSpecBuilder(ipldfree.NodeBuilder())
	layer1Selector, err := ssb.Matcher().Selector()
	require.NoError(t, err)

	recursiveSelector := func(levels int) selector.Selector {
		s, err := ssb.ExploreRecursive(selector.RecursionLimitDepth(levels), ssb.ExploreFields(func(efsb selectorbuilder.ExploreFieldsSpecBuilder) {
			efsb.Insert("parents", ssb.ExploreUnion(
				ssb.ExploreAll(
					ssb.Matcher(),
				),
				ssb.ExploreIndex(0, ssb.ExploreRecursiveEdge()),
			))
		})).Selector()
		require.NoError(t, err)
		return s
	}

	doneAt := func(tsKey block.TipSetKey) func(block.TipSet) (bool, error) {
		return func(ts block.TipSet) (bool, error) {
			if ts.Key().Equals(tsKey) {
				return true, nil
			}
			return false, nil
		}
	}
	withMessageBuilder := func(b *chain.BlockBuilder) {
		b.AddMessages(
			[]*types.SignedMessage{mm.NewSignedMessage(alice, 1)},
			[]*types.UnsignedMessage{},
		)
	}
	withMessageEachBuilder := func(b *chain.BlockBuilder, i int) {
		withMessageBuilder(b)
	}

	verifyNoMessages := func(t *testing.T, ts block.TipSet) {
		for i := 0; i < ts.Len(); i++ {
			blk := ts.At(i)
			stored, err := bs.Has(blk.Messages.SecpRoot)
			require.NoError(t, err)
			require.False(t, stored)
		}
	}

	t.Run("happy path returns correct tipsets", func(t *testing.T) {
		gen := builder.NewGenesis()
		final := builder.BuildOn(gen, 3, withMessageEachBuilder)
		height, err := final.Height()
		require.NoError(t, err)
		chain0 := block.NewChainInfo(pid0, pid0, final.Key(), height)
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		loader := successHeadersLoader(ctx, builder)
		mgs.stubResponseWithLoader(pid0, layer1Selector, loader, final.Key().ToSlice()...)
		mgs.stubResponseWithLoader(pid0, recursiveSelector(1), loader, final.At(0).Cid())

		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))
		done := doneAt(gen.Key())

		ts, err := fetcher.FetchTipSetHeaders(ctx, final.Key(), pid0, done)
		require.NoError(t, err, "the request completes successfully")
		mgs.verifyReceivedRequestCount(4)
		require.Equal(t, 2, len(ts), "the right number of tipsets is returned")
		require.True(t, final.Key().Equals(ts[0].Key()), "the initial tipset is correct")
		require.True(t, gen.Key().Equals(ts[1].Key()), "the remaining tipsets are correct")
		verifyNoMessages(t, ts[0])
		verifyNoMessages(t, ts[1])
	})

	t.Run("fetch succeeds when messages don't decode", func(t *testing.T) {
		mgs := newMockableGraphsync(ctx, bs, fc, t)
		blk := requireSimpleValidBlock(t, 3, address.Undef)
		blk.Messages = types.TxMeta{SecpRoot: notDecodableBlock.Cid(), BLSRoot: types.EmptyMessagesCID}
		key := block.NewTipSetKey(blk.Cid())
		chain0 := block.NewChainInfo(pid0, pid0, key, uint64(blk.Height))
		nd, err := (&types.SignedMessage{}).ToNode()
		require.NoError(t, err)
		notDecodableLoader := simpleLoader([]format.Node{blk.ToNode(), notDecodableBlock, nd})
		mgs.stubResponseWithLoader(pid0, layer1Selector, notDecodableLoader, blk.Cid())
		fetcher := fetcher.NewGraphSyncFetcher(ctx, mgs, bs, bv, fc, newFakePeerTracker(chain0))

		done := doneAt(key)
		ts, err := fetcher.FetchTipSetHeaders(ctx, key, pid0, done)
		assert.NoError(t, err)
		require.Equal(t, 1, len(ts))
		assert.NoError(t, err)
		assert.Equal(t, key, ts[0].Key())
	})
}

func TestRealWorldGraphsyncFetchOnlyHeaders(t *testing.T) {
	tf.IntegrationTest(t)
	ctx := context.Background()
	// setup a chain
	fc := th.NewFakeClock(time.Now())
	genTime := uint64(1234567890)
	chainClock := clock.NewChainClockFromClock(genTime, 5*time.Second, fc)
	builder := chain.NewBuilderWithDeps(t, address.Undef, &chain.FakeStateBuilder{}, chain.NewClockTimestamper(chainClock))
	keys := types.MustGenerateKeyInfo(2, 42)
	mm := types.NewMessageMaker(t, keys)
	alice := mm.Addresses()[0]
	bob := mm.Addresses()[1]
	gen := builder.NewGenesis()

	// count > 64 force multiple layers in amts
	messageCount := uint64(100)

	secpMessages := make([]*types.SignedMessage, messageCount)
	blsMessages := make([]*types.UnsignedMessage, messageCount)
	for i := uint64(0); i < messageCount; i++ {
		secpMessages[i] = mm.NewSignedMessage(alice, i)
		blsMessages[i] = &mm.NewSignedMessage(bob, i).Message
	}

	tipCount := 32
	final := builder.BuildManyOn(tipCount, gen, func(b *chain.BlockBuilder) {
		b.AddMessages(secpMessages, blsMessages)
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

	bv := consensus.NewDefaultBlockValidator(chainClock)
	pt := discovery.NewPeerTracker(peer.ID(""))
	pt.Track(block.NewChainInfo(host2.ID(), host2.ID(), block.TipSetKey{}, 0))

	localLoader := gsstoreutil.LoaderForBlockstore(bs)
	localStorer := gsstoreutil.StorerForBlockstore(bs)

	localGraphsync := graphsync.New(ctx, gsnet1, bridge1, localLoader, localStorer)

	fetcher := fetcher.NewGraphSyncFetcher(ctx, localGraphsync, bs, bv, fc, pt)

	remoteLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		b, err := builder.GetBlockstoreValue(ctx, cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(b.RawData()), nil
	}
	graphsync.New(ctx, gsnet2, bridge2, remoteLoader, nil)

	tipsets, err := fetcher.FetchTipSetHeaders(ctx, final.Key(), host2.ID(), func(ts block.TipSet) (bool, error) {
		if ts.Key().Equals(gen.Key()) {
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)

	require.Equal(t, tipCount+1, len(tipsets))

	// Check the headers are in the store.
	// Check that the messages and receipts are NOT in the store.
	expectedTips := builder.RequireTipSets(final.Key(), tipCount+1)
	for _, ts := range expectedTips {
		stored, err := bs.Has(ts.At(0).Cid())
		require.NoError(t, err)
		assert.True(t, stored)

		stored, err = bs.Has(ts.At(0).Messages.SecpRoot)
		require.NoError(t, err)
		assert.False(t, stored)

		stored, err = bs.Has(ts.At(0).Messages.BLSRoot)
		require.NoError(t, err)
		assert.False(t, stored)

		stored, err = bs.Has(ts.At(0).MessageReceipts)
		require.NoError(t, err)
		assert.False(t, stored)
	}
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
			[]*types.UnsignedMessage{},
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
	fc := th.NewFakeClock(time.Now())
	pt := discovery.NewPeerTracker(peer.ID(""))
	pt.Track(block.NewChainInfo(host2.ID(), host2.ID(), block.TipSetKey{}, 0))

	localLoader := gsstoreutil.LoaderForBlockstore(bs)
	localStorer := gsstoreutil.StorerForBlockstore(bs)

	localGraphsync := graphsync.New(ctx, gsnet1, bridge1, localLoader, localStorer)

	fetcher := fetcher.NewGraphSyncFetcher(ctx, localGraphsync, bs, bv, fc, pt)

	remoteLoader := func(lnk ipld.Link, lnkCtx ipld.LinkContext) (io.Reader, error) {
		cid := lnk.(cidlink.Link).Cid
		node, err := tryBlockstoreValue(ctx, builder, cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(node.RawData()), nil
	}
	graphsync.New(ctx, gsnet2, bridge2, remoteLoader, nil)

	tipsets, err := fetcher.FetchTipSets(ctx, final.Key(), host2.ID(), func(ts block.TipSet) (bool, error) {
		if ts.Key().Equals(gen.Key()) {
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)

	require.Equal(t, tipCount+1, len(tipsets))

	// Check the headers and messages structures are in the store.
	expectedTips := builder.RequireTipSets(final.Key(), tipCount+1)
	for _, ts := range expectedTips {
		stored, err := bs.Has(ts.At(0).Cid())
		require.NoError(t, err)
		assert.True(t, stored)

		stored, err = bs.Has(ts.At(0).Messages.SecpRoot)
		require.NoError(t, err)
		assert.True(t, stored)
	}
}
