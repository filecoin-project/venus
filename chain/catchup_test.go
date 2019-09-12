package chain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

type fakePeerTracker struct {
	updateErr error
	selectErr error
}

func (f *fakePeerTracker) UpdateTrusted(ctx context.Context) error {
	return f.updateErr
}

func (f *fakePeerTracker) SelectHead() (*types.ChainInfo, error) {
	return nil, f.selectErr
}

type fakeSyncer struct {
	hntErr error
}

func (f *fakeSyncer) HandleNewTipSet(ctx context.Context, ci *types.ChainInfo, trusted bool) error {
	return f.hntErr
}

func (f *fakeSyncer) Status() chain.Status {
	return chain.Status{}
}

type fakeStore struct {
	getTsErr error
}

func (f *fakeStore) GetTipSet(key types.TipSetKey) (types.TipSet, error) {
	return types.UndefTipSet, f.getTsErr
}

func (f *fakeStore) GetHead() types.TipSetKey {
	return types.TipSetKey{}
}

func TestCatchUpChainSyncRetry(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	tracker := &fakePeerTracker{
		updateErr: fmt.Errorf("failed to update"),
		selectErr: fmt.Errorf("failed to select"),
	}

	store := &fakeStore{
		getTsErr: fmt.Errorf("failed to get tipset"),
	}

	syncer := &fakeSyncer{
		hntErr: fmt.Errorf("failed to handle new tipset"),
	}

	doneFn := func(newHead *types.ChainInfo, curHead types.TipSet) (bool, error) {
		return false, nil
	}
	catchup := chain.NewCatchupSyncer(tracker, syncer, store, doneFn)

	success, retry, err := catchup.CatchUp(ctx)
	assert.False(t, success)
	assert.True(t, retry)
	assert.Equal(t, fmt.Errorf("failed to update"), err)

	tracker.updateErr = nil

	success, retry, err = catchup.CatchUp(ctx)
	assert.False(t, success)
	assert.True(t, retry)
	assert.Equal(t, fmt.Errorf("failed to select"), err)
}

func TestCatchUpChainFailure(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	tracker := &fakePeerTracker{
		updateErr: nil,
		selectErr: nil,
	}

	store := &fakeStore{
		getTsErr: fmt.Errorf("failed to get tipset"),
	}

	syncer := &fakeSyncer{
		hntErr: fmt.Errorf("failed to handle new tipset"),
	}

	doneFn := func(newHead *types.ChainInfo, curHead types.TipSet) (bool, error) {
		return false, fmt.Errorf("failed to done")
	}

	catchup := chain.NewCatchupSyncer(tracker, syncer, store, doneFn)
	success, retry, err := catchup.CatchUp(ctx)
	assert.False(t, success)
	assert.False(t, retry)
	assert.Equal(t, fmt.Errorf("failed to get tipset"), err)

	store.getTsErr = nil

	success, retry, err = catchup.CatchUp(ctx)
	assert.False(t, success)
	assert.False(t, retry)
	assert.Equal(t, fmt.Errorf("failed to handle new tipset"), err)

	syncer.hntErr = nil

	success, retry, err = catchup.CatchUp(ctx)
	assert.False(t, success)
	assert.False(t, retry)
	assert.Equal(t, fmt.Errorf("failed to done"), err)
}

func TestCatchUpSuccess(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	tracker := &fakePeerTracker{
		updateErr: nil,
		selectErr: nil,
	}

	store := &fakeStore{
		getTsErr: nil,
	}

	syncer := &fakeSyncer{
		hntErr: nil,
	}

	doneFn := func(newHead *types.ChainInfo, curHead types.TipSet) (bool, error) {
		return true, nil
	}

	catchup := chain.NewCatchupSyncer(tracker, syncer, store, doneFn)
	success, retry, err := catchup.CatchUp(ctx)
	assert.True(t, success)
	assert.False(t, retry)
	assert.NoError(t, err)
}
