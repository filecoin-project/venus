package testing

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/shared"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
)

// FakeStateViewer mocks a state viewer for payment channel actor testing
type FakeStateViewer struct {
	t                                                          *testing.T
	actors                                                     map[address.Address]*FakeActorState
	PaychActorPartiesErr, ResolveAddressAtErr, MinerControlErr error
}

// FakeActorState is a mock actor state containing test info
type FakeActorState struct {
	To, From, IDAddr, MinerWorker address.Address
}

// NewFakeStateView initializes a new FakeStateView
func NewFakeStateViewer(t *testing.T) *FakeStateViewer {
	return &FakeStateViewer{
		t:      t,
		actors: make(map[address.Address]*FakeActorState),
	}
}

// MinerControlAddresses mocks returning miner worker and miner actor address
func (f *FakeStateViewer) MinerControlAddresses(_ context.Context, addr address.Address, tok shared.TipSetToken) (owner, worker address.Address, err error) {
	actorState, ok := f.actors[addr]
	if !ok {
		f.t.Fatalf("actor doesn't exist: %s", addr.String())
	}
	return address.Undef, actorState.MinerWorker, f.MinerControlErr
}

// PaychActorParties mocks returning the From and To addrs of a paych.Actor
func (f *FakeStateViewer) PaychActorParties(_ context.Context, paychAddr address.Address, tok shared.TipSetToken) (from, to address.Address, err error) {
	st, ok := f.actors[paychAddr]
	if !ok {
		f.t.Fatalf("actor does not exist %s", paychAddr.String())
	}
	return st.From, st.To, f.PaychActorPartiesErr
}

// AddActorWithState sets up a mock state for actorAddr
func (f *FakeStateViewer) AddActorWithState(actorAddr, from, to, id address.Address) {
	f.actors[actorAddr] = &FakeActorState{to, from, id, address.Undef}
}

// AddMinerWithState sets up a mock state for a miner actor with a worker address
func (f *FakeStateViewer) AddMinerWithState(minerActor, minerWorker address.Address) {
	f.actors[minerActor] = &FakeActorState{MinerWorker: minerWorker}
}

var _ paymentchannel.ActorStateViewer = &FakeStateViewer{}
