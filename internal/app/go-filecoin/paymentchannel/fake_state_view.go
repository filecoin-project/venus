package paymentchannel

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

// FakeChainReader is a mock chain reader
type FakeChainReader struct {
	tsk      block.TipSetKey
	GetTSErr error
}

// GetTipSetStateRoot mocks GetTipSetStateRoot
func (f FakeChainReader) GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error) {
	return f.tsk.ToSlice()[0], f.GetTSErr
}

// Head mocks getting the head TipSetKey.
func (f FakeChainReader) Head() block.TipSetKey {
	return f.tsk
}

// NewFakeChainReader initializes a new FakeChainReader with `tsk`
func NewFakeChainReader(tsk block.TipSetKey) *FakeChainReader {
	return &FakeChainReader{tsk: tsk}
}

var _ ChainReader = &FakeChainReader{}

// FakeStateViewer mocks a state viewer for payment channel actor testing
type FakeStateViewer struct {
	Views map[cid.Cid]*FakeStateView
}

// StateView mocks fetching a real state view
func (f *FakeStateViewer) StateView(root cid.Cid) PaychActorStateView {
	return f.Views[root]
}

// FakeStateView is a mock version of a state view for payment channel actors
type FakeStateView struct {
	t                                                          *testing.T
	actors                                                     map[address.Address]*FakeActorState
	PaychActorPartiesErr, ResolveAddressAtErr, MinerControlErr error
}

func (f *FakeStateView) MinerControlAddresses(_ context.Context, addr address.Address) (owner, worker address.Address, err error) {
	actorState, ok := f.actors[addr]
	if !ok {
		f.t.Fatalf("actor doesn't exist: %s", addr.String())
	}
	return address.Undef, actorState.MinerWorker, f.MinerControlErr
}

// NewFakeStateView initializes a new FakeStateView
func NewFakeStateView(t *testing.T, viewErr error) *FakeStateView {
	return &FakeStateView{
		t:                    t,
		actors:               make(map[address.Address]*FakeActorState),
		PaychActorPartiesErr: viewErr,
	}
}

// PaychActorParties mocks returning the From and To addrs of a paych.Actor
func (f *FakeStateView) PaychActorParties(_ context.Context, paychAddr address.Address) (from, to address.Address, err error) {
	st, ok := f.actors[paychAddr]
	if !ok {
		f.t.Fatalf("actor does not exist %s", paychAddr.String())
	}
	return st.From, st.To, f.PaychActorPartiesErr
}

// AddActorWithState sets up a mock state for actorAddr
func (f *FakeStateView) AddActorWithState(actorAddr, from, to, id address.Address) {
	f.actors[actorAddr] = &FakeActorState{to, from, id, address.Undef}
}

func (f *FakeStateView) AddMinerWithState(minerActor, minerWorker address.Address) {
	f.actors[minerActor] = &FakeActorState{ MinerWorker: minerWorker }
}

var _ PaychActorStateView = &FakeStateView{}

// FakeActorState is a mock actor state containing test info
type FakeActorState struct {
	To, From, IDAddr, MinerWorker address.Address
}
