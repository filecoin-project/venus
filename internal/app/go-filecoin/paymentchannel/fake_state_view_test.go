package paymentchannel_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paymentchannel"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
)

type FakeChainReader struct {
	tsk      block.TipSetKey
	GetTSErr error
}

func (f FakeChainReader) GetTipSetStateRoot(context.Context, block.TipSetKey) (cid.Cid, error) {
	return f.tsk.ToSlice()[0], f.GetTSErr
}

func (f FakeChainReader) Head() block.TipSetKey {
	return f.tsk
}

func NewFakeChainReader(tsk block.TipSetKey) *FakeChainReader {
	return &FakeChainReader{tsk: tsk}
}

var _ paymentchannel.ChainReader = &FakeChainReader{}

// FakeStateViewer mocks a state viewer for payment channel actor testing
type FakeStateViewer struct {
	Views map[cid.Cid]*FakeStateView
}

// StateView mocks fetching a real state view
func (f *FakeStateViewer) StateView(root cid.Cid) paymentchannel.PaychActorStateView {
	return f.Views[root]
}

// FakeStateView is a mock version of a state view for payment channel actors
type FakeStateView struct {
	t                                         *testing.T
	actors                                    map[address.Address]*FakeActorState
	PaychActorPartiesErr, ResolveAddressAtErr error
}

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
func (f *FakeStateView) ResolveAddressAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (address.Address, error) {
	st, ok := f.actors[addr]
	if !ok {
		f.t.Fatalf("actor does not exist %s", addr.String())
	}
	return st.IDAddr, f.ResolveAddressAtErr
}

func (f *FakeStateView) AddActorWithState(actorAddr, from, to, id address.Address) {
	f.actors[actorAddr] = &FakeActorState{to, from, id}
}

var _ paymentchannel.PaychActorStateView = &FakeStateView{}

// FakeActorState is a mock actor state containing test info
type FakeActorState struct {
	To, From, IDAddr address.Address
}
