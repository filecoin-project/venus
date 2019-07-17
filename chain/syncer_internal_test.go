package chain

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/repo"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	log.SetDebugLogging()
}

func TestSyncerBootstrapMode(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()
	cb := NewBuilder(t, address.Undef)
	gen := cb.AppendOn()
	genTs := types.RequireNewTipSet(t, gen)
	t.Logf("Genesis CID: %s", genTs.String())

	// Persist the genesis tipset to the repo.
	genTsas := &TipSetAndState{
		TipSet:          genTs,
		TipSetStateRoot: types.SomeCid(),
	}

	r := repo.NewInMemoryRepo()
	chainDS := r.ChainDatastore()
	chainStore := NewStore(chainDS, &fakeCst{}, gen.Cid())
	chainStore.PutTipSetAndState(ctx, genTsas)
	chainStore.SetHead(ctx, genTs)

	fpm := &fakePeerManager{}
	syncer := NewSyncer(&fakeStateEvaluator{}, chainStore, cb, fpm, Syncing)

	b10 := cb.AppendManyOn(9, gen)
	fpm.head = types.RequireNewTipSet(t, b10)

	err := syncer.SyncBootstrap(ctx)
	assert.NoError(t, err)

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
