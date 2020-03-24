package paymentchannel

import (
	"context"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/support/mock"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakeActorInterface fulfils the ActorStateViewer and MsgSender interfaces for a Manager
// via the specs_actors mock runtime. It executes actor exports directly.
type FakeActorInterface struct {
	t   *testing.T
	ctx context.Context
	*mock.Runtime
	*chain.Builder
	head      block.TipSetKey                  // Provided by GetHead and expected by others
	actors    map[address.Address]*actor.Actor // for actors not under test
	receivers map[address.Address]receiver
}

type receiver interface {
	code() cid.Cid
	execAndVerify(t *testing.T, rt *mock.Runtime, codeID cid.Cid, params []byte)
}

func NewFakeActorInterface(t *testing.T, ctx context.Context, rtbuilder *mock.RuntimeBuilder, chainBuilder *chain.Builder) *FakeActorInterface {
	return &FakeActorInterface{
		ctx:       ctx,
		Builder:   chainBuilder,
		Runtime:   rtbuilder.Build(t),
		receivers: make(map[address.Address]receiver),
		t:         t,
	}
}

func (fms *FakeActorInterface) GetState(typ reflect.Type) interface{} {
	st := reflect.New(typ.Elem()).Interface()
	unm, ok := st.(runtime.CBORUnmarshaler)
	require.True(fms.t, ok)
	fms.Runtime.GetState(unm)
	return st
}

// Send posts a message to the chain
func (fms *FakeActorInterface) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	return cid.Undef, nil, nil
}

// ConstructActor calls the constructor for an actor that will be the recipient of the Send
// messages
func (fms *FakeActorInterface) ConstructActor(actorType cid.Cid, newActor, caller address.Address, balance abi.TokenAmount) {
	switch actorType {
	case builtin.InitActorCodeID:
		fms.constructInitActor(newActor)
	case builtin.PaymentChannelActorCodeID:
		fms.constructPaymentChannelActor(newActor, caller, balance)
	}
	fms.Runtime.Verify()
}

func (fms *FakeActorInterface) SetHead(head block.TipSetKey) {
	_, e := fms.GetTipSet(head)
	require.NoError(fms.t, e)
	fms.head = head
}

// SetActor sets an actor to be mocked on chain
func (fms *FakeActorInterface) SetActor(addr address.Address, act *actor.Actor) {
	fms.actors[addr] = act
}

func (fms *FakeActorInterface) ExecAndVerify(caller, receiver address.Address, code cid.Cid, params []byte) {
	a, ok := fms.receivers[receiver]
	if !ok {
		fms.t.Fatalf("actor does not exist %s", receiver.String())
	}
	fms.Runtime.SetCaller(caller, builtin.AccountActorCodeID)

	a.execAndVerify(fms.t, fms.Runtime, code, params)
}

// this constructs an initActor harness with the fms mock runtime, so that initActor exports
// can be tested in go-filecoin.
func (fms *FakeActorInterface) constructInitActor(addr address.Address) {
	fms.Runtime.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	fms.Runtime.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	h := initActorHarness{}
	ret := fms.Runtime.Call(h.Constructor, &init_.ConstructorParams{NetworkName: "mock"})
	require.Nil(fms.t, ret)
	fms.receivers[addr] = &h
}

// this constructs a payment channel actor harness with the fms mock runtime, so that paychActor exports
// can be tested in go-filecoin.
//  If you want to test creation of an actor by InitActor use ExecAndVerify
func (fms *FakeActorInterface) constructPaymentChannelActor(newActor, caller address.Address, balance abi.TokenAmount) {

}

type initActorHarness struct {
	init_.Actor
	t testing.TB
}

func (h *initActorHarness) code() cid.Cid {
	return builtin.InitActorCodeID
}

func (h *initActorHarness) execAndVerify(t *testing.T, rt *mock.Runtime, codeID cid.Cid, params []byte) {
	rt.ExpectValidateCallerAny()
	res := rt.Call(h.Exec, &init_.ExecParams{
		CodeCID:           codeID,
		ConstructorParams: params,
	}).(*init_.ExecReturn)
	require.NotNil(t, res)
	rt.Verify()
}
