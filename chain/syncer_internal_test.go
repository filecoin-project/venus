package chain

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	log.SetDebugLogging()
}

func setupChainBuilderAndStoreWithGenesisState(t *testing.T) (*types.Block, *Builder, *Store) {
	ctx := context.Background()
	cb := NewBuilder(t, address.Undef)
	gen := cb.AppendOn()
	genTs := types.RequireNewTipSet(t, gen)

	// Persist the genesis tipset to the repo.
	genTsas := &TipSetAndState{
		TipSet:          genTs,
		TipSetStateRoot: types.SomeCid(),
	}

	r := repo.NewInMemoryRepo()
	chainDS := r.ChainDatastore()
	chainStore := NewStore(chainDS, &fakeCst{}, &fakeTreeLoader{}, gen.Cid())
	chainStore.PutTipSetAndState(ctx, genTsas)
	chainStore.SetHead(ctx, genTs)

	return gen, cb, chainStore
}

func TestSimpleBootstrapMode(t *testing.T) {
	tf.UnitTest(t)

	gen, cb, chainStore := setupChainBuilderAndStoreWithGenesisState(t)
	ctx := context.Background()

	fpm := &fakePeerManager{}
	syncer := NewSyncer(&fakeStateEvaluator{}, chainStore, cb, fpm, Syncing)

	b10 := cb.AppendManyOn(9, gen)
	fpm.head = types.RequireNewTipSet(t, b10)

	// assert the syncer:
	// - Bootstrapped successfully
	// - didn't encounter any bad tipset
	// - has the expected head
	// - is now in caught up mode
	assert.NoError(t, syncer.SyncBootstrap(ctx))
	assert.Equal(t, 0, len(syncer.badTipSets.bad))
	assert.Equal(t, types.RequireNewTipSet(t, b10).Key(), chainStore.GetHead())
	assert.Equal(t, CaughtUp, syncer.syncMode)
}

func TestSimpleCaughtUpMode(t *testing.T) {
	tf.UnitTest(t)

	gen, cb, chainStore := setupChainBuilderAndStoreWithGenesisState(t)
	ctx := context.Background()

	fpm := &fakePeerManager{}
	syncer := NewSyncer(&fakeStateEvaluator{}, chainStore, cb, fpm, CaughtUp)

	b10 := cb.AppendManyOn(9, gen)
	cb.AppendManyOn(9, gen)
	for _, blk := range cb.blocks {
		chainStore.PutTipSetAndState(ctx, &TipSetAndState{
			TipSet:          types.RequireNewTipSet(t, blk),
			TipSetStateRoot: types.SomeCid(),
		})
	}
	require.NoError(t, chainStore.SetHead(ctx, types.RequireNewTipSet(t, b10)))

	b11 := cb.AppendOn(b10)

	// assert the syncer:
	// - CaughtUp successfully
	// - didn't encounter any bad tipset
	// - has the expected head
	// - is still in caughtup mode
	assert.NoError(t, syncer.SyncCaughtUp(ctx, types.RequireNewTipSet(t, b11).Key()))
	assert.Equal(t, 0, len(syncer.badTipSets.bad))
	assert.Equal(t, types.RequireNewTipSet(t, b11).Key(), chainStore.GetHead())
	assert.Equal(t, CaughtUp, syncer.syncMode)

	// craft a block too far in the future
	outOfFinalityBlock := cb.BuildOn(b11, func(b *BlockBuilder) {
		b.IncHeight(types.Uint64(FinalityLimit))
	})

	// block is rejected
	expectFinalityErr := syncer.SyncCaughtUp(ctx, types.RequireNewTipSet(t, outOfFinalityBlock).Key())
	assert.Equal(t, ErrNewChainTooLong, expectFinalityErr)

	// craft a block one back from finality
	outOfFinalityBlock = cb.BuildOn(b11, func(b *BlockBuilder) {
		b.IncHeight(types.Uint64(FinalityLimit - 1))
	})

	// block is accepts
	noErr := syncer.SyncCaughtUp(ctx, types.RequireNewTipSet(t, outOfFinalityBlock).Key())
	assert.NoError(t, noErr)
}

type fakeTreeLoader struct {
}

func (ftl *fakeTreeLoader) LoadStateTree(ctx context.Context, store state.IpldStore, c cid.Cid, builtinActors map[cid.Cid]exec.ExecutableActor) (state.Tree, error) {
	return nil, nil
}

type fakeCst struct {
}

func (s *fakeCst) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	return nil
}
func (s *fakeCst) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	return types.SomeCid(), nil
}

type fakePeerManager struct {
	head types.TipSet
}

// AddPeer adds a peer to the peer managers store.
func (f *fakePeerManager) AddPeer(p peer.ID, h types.TipSet) {
	panic("not implemented")
}

// Peers returns the peers the peer manager is aware of.
func (f *fakePeerManager) Peers() []peer.ID {
	panic("not implemented")
}

// PeerHeads returns all peers and their heads the peer manager knows about.
func (f *fakePeerManager) PeerHeads() map[peer.ID]types.TipSet {
	panic("not implemented")
}

// Select computes the best head from the peers its aware of.
func (f *fakePeerManager) SelectHead() (types.TipSet, error) {
	return f.head, nil
}

type fakeStateEvaluator struct {
}

// prior `stateRoot`.  It returns an error if the transition is invalid.
func (f *fakeStateEvaluator) RunStateTransition(ctx context.Context, ts types.TipSet, ancestors []types.TipSet, stateID cid.Cid) (cid.Cid, error) {
	return types.SomeCid(), nil
}

// IsHeaver returns 1 if tipset a is heavier than tipset b and -1 if
// tipset b is heavier than tipset a.
func (f *fakeStateEvaluator) IsHeavier(ctx context.Context, a, b types.TipSet, aStateID, bStateID cid.Cid) (bool, error) {
	return true, nil
}
