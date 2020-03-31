package testing

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
)

type FakeStateViewer struct {
	view *FakeStateView
}

// NewFakeStateViewer initializes a new FakeStateViewer
func NewFakeStateViewer(t *testing.T) *FakeStateViewer {
	view := &FakeStateView{
		t:      t,
		actors: make(map[address.Address]*FakeActorState),
	}
	return &FakeStateViewer{view: view}
}

// GetStateView returns the fake state view as a paymentchannel.ManagerStateView interface
func (f FakeStateViewer) GetStateView(ctx context.Context, tok shared.TipSetToken) (paymentchannel.ManagerStateView, error) {
	return f.view, nil
}

// GetFakeStateView returns the FakeStateView as itself so test setup can be done.
func (f FakeStateViewer) GetFakeStateView() *FakeStateView {
	return f.view
}

// FakeStateView mocks a state view for payment channel actor testing
type FakeStateView struct {
	t                                                          *testing.T
	actors                                                     map[address.Address]*FakeActorState
	PaychActorPartiesErr, ResolveAddressAtErr, MinerControlErr error
}

var _ paymentchannel.ManagerStateView = new(FakeStateView)

// FakeActorState is a mock actor state containing test info
type FakeActorState struct {
	To, From, IDAddr, MinerWorker address.Address
}

// MinerControlAddresses mocks returning miner worker and miner actor address
func (f *FakeStateView) MinerControlAddresses(_ context.Context, addr address.Address) (owner, worker address.Address, err error) {
	actorState, ok := f.actors[addr]
	if !ok {
		f.t.Fatalf("actor doesn't exist: %s", addr.String())
	}
	return address.Undef, actorState.MinerWorker, f.MinerControlErr
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

// AddMinerWithState sets up a mock state for a miner actor with a worker address
func (f *FakeStateView) AddMinerWithState(minerActor, minerWorker address.Address) {
	f.actors[minerActor] = &FakeActorState{MinerWorker: minerWorker}
}

var _ paymentchannel.ActorStateViewer = &FakeStateViewer{}
