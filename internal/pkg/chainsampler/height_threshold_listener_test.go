package chainsampler

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/specactors/policy"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestNewHeightThresholdListener(t *testing.T) {
	tf.UnitTest(t)
	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.Genesis()

	startHead := builder.BuildManyOn(6, genesis, nil)

	t.Run("does nothing until chain crosses threshold", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		hc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(11, hc, ec, ic, dc)

		// 6 + 3 = 9 which is less than 11
		nextTS := builder.BuildManyOn(3, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(newChain)
			require.NoError(t, err)
			cancel()
		}()

		expectCancelBeforeOutput(ctx, hc, ec, ic, dc)
	})

	t.Run("add tipset at target height sends key", func(t *testing.T) {
		hc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(7, hc, ec, ic, dc)

		nextTS := builder.Build(startHead, 1, nil)
		go func() {
			_, err := listener.Handle([]*block.TipSet{nextTS})
			require.NoError(t, err)
		}()

		key := waitForKey(t, hc, ec, ic, dc)
		assert.Equal(t, nextTS.Key(), key)
	})

	t.Run("invalidates when new fork head is lower than target", func(t *testing.T) {
		hc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, hc, ec, ic, dc)

		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(newChain)
			require.NoError(t, err)
		}()

		key := waitForKey(t, hc, ec, ic, dc)
		assert.Equal(t, newChain[2].Key(), key)

		shorterFork := builder.BuildManyOn(1, startHead, nil)
		go func() {
			_, err := listener.Handle([]*block.TipSet{shorterFork})
			require.NoError(t, err)
		}()

		waitForInvalidation(t, hc, ec, ic, dc)
	})

	t.Run("invalidates and then sends new seed when new fork head is higher than target with a lower lca", func(t *testing.T) {
		hc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, hc, ec, ic, dc)

		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(newChain)
			require.NoError(t, err)
		}()

		key := waitForKey(t, hc, ec, ic, dc)
		assert.Equal(t, newChain[2].Key(), key)

		fork := builder.BuildManyOn(3, startHead, nil)
		forkSlice, err := tipsetToSlice(fork, 3, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(forkSlice)
			require.NoError(t, err)
		}()

		// first invalidate
		waitForInvalidation(t, hc, ec, ic, dc)

		// then send new key
		key = waitForKey(t, hc, ec, ic, dc)
		assert.Equal(t, forkSlice[1].Key(), key)
	})

	t.Run("does nothing if new chain is entirely above threshold", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		hc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, hc, ec, ic, dc)

		// cross the threshold (8) with 4 tipsets
		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(newChain)
			require.NoError(t, err)
		}()

		key := waitForKey(t, hc, ec, ic, dc)
		assert.Equal(t, newChain[2].Key(), key)

		// add 3 more tipsets on existing highest head that do not cross threshold
		nextTS = builder.BuildManyOn(3, nextTS, nil)
		newChain, err = tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(newChain)
			require.NoError(t, err)
			cancel()
		}()

		expectCancelBeforeOutput(ctx, hc, ec, ic, dc)
	})

	t.Run("sends on done channel when finality is crossed", func(t *testing.T) {
		hc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, hc, ec, ic, dc)

		// cross the threshold (8) with 4 tipsets
		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(newChain)
			require.NoError(t, err)
		}()

		key := waitForKey(t, hc, ec, ic, dc)
		assert.Equal(t, newChain[2].Key(), key)

		// add tipsets till finality
		go func() {
			for i := abi.ChainEpoch(0); i < policy.ChainFinality; i++ {
				nextTS = builder.BuildOn(nextTS, 1, nil)
				valid, err := listener.Handle([]*block.TipSet{nextTS})
				require.NoError(t, err)

				h, err := nextTS.Height()
				require.NoError(t, err)
				if h >= 8+policy.ChainFinality {
					assert.False(t, valid)
				} else {
					assert.True(t, valid)
				}
			}
		}()

		select {
		case <-hc:
			panic("unexpected sample key")
		case err := <-ec:
			panic(err)
		case <-ic:
			panic("unexpected height invalidation")
		case <-dc:
			return // got value on done channel
		}
	})
}

func setupChannels() (chan block.TipSetKey, chan error, chan struct{}, chan struct{}) {
	return make(chan block.TipSetKey), make(chan error), make(chan struct{}), make(chan struct{})
}

func waitForKey(t *testing.T, hc chan block.TipSetKey, ec chan error, ic, dc chan struct{}) block.TipSetKey {
	select {
	case key := <-hc:
		return key
	case err := <-ec:
		panic(err)
	case <-ic:
		panic("unexpected height invalidation")
	case <-dc:
		panic("listener completed before sending key")
	}
}

func expectCancelBeforeOutput(ctx context.Context, hc chan block.TipSetKey, ec chan error, ic, dc chan struct{}) {
	select {
	case <-hc:
		panic("unexpected target tip set")
	case err := <-ec:
		panic(err)
	case <-ic:
		panic("unexpected height invalidation")
	case <-dc:
		panic("listener completed before sending seed")
	case <-ctx.Done():
		return
	}
}

func waitForInvalidation(t *testing.T, hc chan block.TipSetKey, ec chan error, ic, dc chan struct{}) {
	select {
	case <-hc:
		panic("got key when we expected invalidation")
	case err := <-ec:
		panic(err)
	case <-ic:
		return
	case <-dc:
		panic("listener completed before sending key")
	}
}

func tipsetToSlice(ts *block.TipSet, ancestors int, builder *chain.Builder) ([]*block.TipSet, error) {
	s := make([]*block.TipSet, ancestors)
	for i := 0; i < ancestors; i++ {
		s[i] = ts

		tskey, err := ts.Parents()
		if err != nil {
			return nil, err
		}

		ts, err = builder.GetTipSet(tskey)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}
