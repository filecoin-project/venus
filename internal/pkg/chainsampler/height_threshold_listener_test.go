package chainsampler

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-storage-miner"
)

func TestNewHeightThresholdListener(t *testing.T) {
	builder := chain.NewBuilder(t, address.Undef)
	genesis := builder.NewGenesis()

	startHead := builder.BuildManyOn(6, genesis, nil)

	t.Run("does nothing until chain crosses threshold", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		sc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(11, sc, ic, dc, ec)

		sampler := func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
			return sampleHeight.Bytes(), nil
		}

		// 6 + 3 = 9 which is less than 11
		nextTS := builder.BuildManyOn(3, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(ctx, newChain, sampler)
			require.NoError(t, err)
			cancel()
		}()

		expectCancelBeforeOutput(ctx, sc, ec, ic, dc)
	})

	t.Run("add tipset at target height sends seed", func(t *testing.T) {
		sc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(7, sc, ic, dc, ec)

		sampler := func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
			return sampleHeight.Bytes(), nil
		}

		nextTS := builder.Build(startHead, 1, nil)
		go func() {
			_, err := listener.Handle(context.TODO(), []block.TipSet{nextTS}, sampler)
			require.NoError(t, err)
		}()

		seed := waitForSeed(t, sc, ec, ic, dc)
		assert.Equal(t, uint64(7), seed.BlockHeight)
		assert.Equal(t, types.NewBlockHeight(7).Bytes(), seed.TicketBytes)
	})

	t.Run("invalidates when new fork head is lower than target", func(t *testing.T) {
		sc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, sc, ic, dc, ec)

		sampler := func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
			return sampleHeight.Bytes(), nil
		}

		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(context.TODO(), newChain, sampler)
			require.NoError(t, err)
		}()

		seed := waitForSeed(t, sc, ec, ic, dc)
		assert.Equal(t, uint64(8), seed.BlockHeight)

		shorterFork := builder.BuildManyOn(1, startHead, nil)
		go func() {
			_, err := listener.Handle(context.TODO(), []block.TipSet{shorterFork}, sampler)
			require.NoError(t, err)
		}()

		waitForInvalidation(t, sc, ec, ic, dc)
	})

	t.Run("invalidates and then sends new seed when new fork head is higher than target with a lower lca", func(t *testing.T) {
		sc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, sc, ic, dc, ec)

		sampler := func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
			return sampleHeight.Bytes(), nil
		}

		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(context.TODO(), newChain, sampler)
			require.NoError(t, err)
		}()

		seed := waitForSeed(t, sc, ec, ic, dc)
		assert.Equal(t, uint64(8), seed.BlockHeight)

		fork := builder.BuildManyOn(3, startHead, nil)
		forkSlice, err := tipsetToSlice(fork, 3, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(context.TODO(), forkSlice, sampler)
			require.NoError(t, err)
		}()

		// first invalidate
		waitForInvalidation(t, sc, ec, ic, dc)

		// then send new seed
		seed = waitForSeed(t, sc, ec, ic, dc)
		assert.Equal(t, uint64(8), seed.BlockHeight)
		assert.Equal(t, types.NewBlockHeight(8).Bytes(), seed.TicketBytes)
	})

	t.Run("does nothing if new chain is entirely above threshold", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		sc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, sc, ic, dc, ec)

		sampler := func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
			return sampleHeight.Bytes(), nil
		}

		// cross the threshold (8) with 4 tipsets
		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(context.TODO(), newChain, sampler)
			require.NoError(t, err)
		}()

		seed := waitForSeed(t, sc, ec, ic, dc)
		assert.Equal(t, uint64(8), seed.BlockHeight)

		// add 3 more tipsets on existing highest head that do not cross threshold
		nextTS = builder.BuildManyOn(3, nextTS, nil)
		newChain, err = tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(ctx, newChain, sampler)
			require.NoError(t, err)
			cancel()
		}()

		expectCancelBeforeOutput(ctx, sc, ec, ic, dc)
	})

	t.Run("sends on done channel when finality is crossed", func(t *testing.T) {
		sc, ec, ic, dc := setupChannels()
		listener := NewHeightThresholdListener(8, sc, ic, dc, ec)

		sampler := func(ctx context.Context, sampleHeight *types.BlockHeight) ([]byte, error) {
			return sampleHeight.Bytes(), nil
		}

		// cross the threshold (8) with 4 tipsets
		nextTS := builder.BuildManyOn(4, startHead, nil)
		newChain, err := tipsetToSlice(nextTS, 4, builder)
		require.NoError(t, err)

		go func() {
			_, err := listener.Handle(context.TODO(), newChain, sampler)
			require.NoError(t, err)
		}()

		seed := waitForSeed(t, sc, ec, ic, dc)
		assert.Equal(t, uint64(8), seed.BlockHeight)

		// add tipsets till finality
		go func() {
			for i := 0; i < consensus.FinalityEpochs; i++ {
				nextTS = builder.BuildOn(nextTS, 1, nil)
				valid, err := listener.Handle(context.TODO(), []block.TipSet{nextTS}, sampler)
				require.NoError(t, err)

				h, err := nextTS.Height()
				require.NoError(t, err)
				if h >= 8+consensus.FinalityEpochs {
					assert.False(t, valid)
				} else {
					assert.True(t, valid)
				}
			}
		}()

		select {
		case <-sc:
			panic("unexpected sample seed")
		case err := <-ec:
			panic(err)
		case <-ic:
			panic("unexpected height invalidation")
		case <-dc:
			return // got value on done channel
		}
	})
}

func setupChannels() (chan storage.SealSeed, chan error, chan struct{}, chan struct{}) {
	return make(chan storage.SealSeed), make(chan error), make(chan struct{}), make(chan struct{})
}

func waitForSeed(t *testing.T, sc chan storage.SealSeed, ec chan error, ic, dc chan struct{}) storage.SealSeed {
	select {
	case seed := <-sc:
		return seed
	case err := <-ec:
		panic(err)
	case <-ic:
		panic("unexpected height invalidation")
	case <-dc:
		panic("listener completed before sending seed")
	}
}

func expectCancelBeforeOutput(ctx context.Context, sc chan storage.SealSeed, ec chan error, ic, dc chan struct{}) {
	select {
	case <-sc:
		panic("unexpected sample seed")
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

func waitForInvalidation(t *testing.T, sc chan storage.SealSeed, ec chan error, ic, dc chan struct{}) {
	select {
	case <-sc:
		panic("got seed when we expected invalidation")
	case err := <-ec:
		panic(err)
	case <-ic:
		return
	case <-dc:
		panic("listener completed before sending seed")
	}
}

func tipsetToSlice(ts block.TipSet, ancestors int, builder *chain.Builder) ([]block.TipSet, error) {
	s := make([]block.TipSet, ancestors)
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
