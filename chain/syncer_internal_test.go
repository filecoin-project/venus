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

// minerAddr can be thought of as a "seed". Different values for minerAddr will produce
// blocks with different CID's
func setupChainBuilderAndStoreWithGenesisState(t *testing.T, minerAddr address.Address) (*types.Block, *Builder, *Store) {
	ctx := context.Background()
	cb := NewBuilder(t, minerAddr)
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
	require.NoError(t, chainStore.PutTipSetAndState(ctx, genTsas))
	require.NoError(t, chainStore.SetHead(ctx, genTs))

	return gen, cb, chainStore
}

func TestBootstrapModeInitialNetworkJoin(t *testing.T) {
	tf.UnitTest(t)

	gen, cb, chainStore := setupChainBuilderAndStoreWithGenesisState(t, address.Undef)
	ctx := context.Background()

	fpm := &fakePeerManager{}
	processor := NewChainStateEvaluator(&fakeStateEvaluator{}, chainStore)
	syncer := NewSyncer(processor, chainStore, cb, fpm, Syncing)

	b10 := cb.AppendManyOn(9, gen)
	fpm.head = types.RequireNewTipSet(t, b10)

	// assert the syncer:
	// - Bootstrapped successfully
	// - didn't encounter any bad tipset
	// - has the expected head
	// - is now in caught up mode
	assert.NoError(t, syncer.SyncBootstrap(ctx))
	assert.Equal(t, types.RequireNewTipSet(t, b10).Key(), chainStore.GetHead())
	assert.Equal(t, CaughtUp, syncer.syncMode)
}

func TestBootstrapModeInvalidGenesisBlock(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	minerA, err := address.NewActorAddress([]byte("minerA"))
	require.NoError(t, err)
	minerB, err := address.NewActorAddress([]byte("minerB"))
	require.NoError(t, err)

	genA, cbA, chainStoreA := setupChainBuilderAndStoreWithGenesisState(t, minerA)
	genB, cbB, _ := setupChainBuilderAndStoreWithGenesisState(t, minerB)

	fpm := &fakePeerManager{}
	processor := NewChainStateEvaluator(&fakeStateEvaluator{}, chainStoreA)
	syncer := NewSyncer(processor, chainStoreA, cbB, fpm, Syncing)

	b10B := cbB.AppendManyOn(9, genB)
	fpm.head = types.RequireNewTipSet(t, b10B)
	assert.Contains(t, syncer.SyncBootstrap(ctx).Error(), "wrong genesis")

	// now ensure we can sync to a known block
	b10A := cbA.AppendManyOn(9, genA)
	fpm.head = types.RequireNewTipSet(t, b10A)
	syncer.fetcher = cbA // this feels kinda hacky, works for now though
	assert.NoError(t, syncer.SyncBootstrap(ctx))

}

// TODO this test needs a better name
func TestBootstrapModeAfterRestartSimple(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	gen, cb, chainStore := setupChainBuilderAndStoreWithGenesisState(t, address.Undef)

	cur := gen
	for i := 0; i < 9; i++ {
		blk := cb.AppendOn(cur)
		blkTs := types.RequireNewTipSet(t, blk)
		blkTsas := &TipSetAndState{
			TipSet:          blkTs,
			TipSetStateRoot: types.SomeCid(),
		}
		require.NoError(t, chainStore.PutTipSetAndState(ctx, blkTsas))
		cur = blk
	}

	fpm := &fakePeerManager{}
	fse := &fakeStateEvaluator{}
	processor := NewChainStateEvaluator(fse, chainStore)
	syncer := NewSyncer(processor, chainStore, cb, fpm, Syncing)

	// create blocks that are not in the syncer chainstore
	b20 := cb.AppendManyOn(9, cur)
	fpm.head = types.RequireNewTipSet(t, b20)

	// TODO add an assertion that the Evaluate method was only called 9 times
	assert.NoError(t, syncer.SyncBootstrap(ctx))
	assert.Equal(t, CaughtUp, syncer.syncMode)

}

// testing the case when the node is restarted and the chain forks on a previous
// tipset the node had.
func TestBootstrapModeAfterRestartForked(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	forkMinerAddr, err := address.NewActorAddress([]byte("forkMiner"))
	require.NoError(t, err)

	gen, cb, chainStore := setupChainBuilderAndStoreWithGenesisState(t, address.Undef)

	cur := gen
	var fork *types.Block
	for i := 0; i < 9; i++ {
		blk := cb.AppendOn(cur)
		blkTs := types.RequireNewTipSet(t, blk)
		blkTsas := &TipSetAndState{
			TipSet:          blkTs,
			TipSetStateRoot: types.SomeCid(),
		}
		require.NoError(t, chainStore.PutTipSetAndState(ctx, blkTsas))
		if i == 3 {
			fork = blk
			t.Logf("Fork Block: %s", fork.Cid())
		}
		cur = blk
		t.Logf("StorePut %s", cur.Cid())
	}
	t.Logf("Old Head: %s", cur.Cid())

	fpm := &fakePeerManager{}
	fse := &fakeStateEvaluator{}
	processor := NewChainStateEvaluator(fse, chainStore)

	// create blocks that are not in the syncer chainstore
	cbF := NewBuilder(t, forkMinerAddr)
	b20 := cbF.AppendManyOn(9, fork)

	// need to merge the chainBuilders so fetching works
	for k, v := range cb.blocks {
		cbF.blocks[k] = v
	}

	fpm.head = types.RequireNewTipSet(t, b20)
	syncer := NewSyncer(processor, chainStore, cbF, fpm, Syncing)

	// TODO add an assertion that the Evaluate method was only called 9 times
	assert.NoError(t, syncer.SyncBootstrap(ctx))
	assert.Equal(t, CaughtUp, syncer.syncMode)

}

func TestSimpleCaughtUpMode(t *testing.T) {
	tf.UnitTest(t)

	gen, cb, chainStore := setupChainBuilderAndStoreWithGenesisState(t, address.Undef)
	ctx := context.Background()

	fpm := &fakePeerManager{}
	processor := NewChainStateEvaluator(&fakeStateEvaluator{}, chainStore)
	syncer := NewSyncer(processor, chainStore, cb, fpm, CaughtUp)

	b10 := cb.AppendManyOn(9, gen)
	cb.AppendManyOn(9, gen)
	for _, blk := range cb.blocks {
		require.NoError(t, chainStore.PutTipSetAndState(ctx, &TipSetAndState{
			TipSet:          types.RequireNewTipSet(t, blk),
			TipSetStateRoot: types.SomeCid(),
		}))
	}
	require.NoError(t, chainStore.SetHead(ctx, types.RequireNewTipSet(t, b10)))

	b11 := cb.AppendOn(b10)

	// assert the syncer:
	// - CaughtUp successfully
	// - didn't encounter any bad tipset
	// - has the expected head
	// - is still in caughtup mode
	assert.NoError(t, syncer.SyncCaughtUp(ctx, types.RequireNewTipSet(t, b11).Key()))
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
