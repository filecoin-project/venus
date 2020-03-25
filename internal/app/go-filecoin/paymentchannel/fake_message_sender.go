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

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakeMessageSender is a message sender that parses message parameters and invokes the
// export directly on the actor.
// It uses the specs_actors mock runtime.
type FakeMessageSender struct {
	t         *testing.T
	ctx       context.Context
	rt        *mock.Runtime
	receivers map[address.Address]receiver
}

type receiver interface {
	code() cid.Cid
	execAndVerify(t *testing.T, rt *mock.Runtime, codeID cid.Cid, params []byte)
}

func NewFakeMessageSender(t *testing.T, ctx context.Context, builder *mock.RuntimeBuilder) *FakeMessageSender {
	return &FakeMessageSender{
		ctx:       ctx,
		rt:        builder.Build(t),
		receivers: make(map[address.Address]receiver),
		t:         t,
	}
}

func (fms *FakeMessageSender) Runtime() *mock.Runtime {
	return fms.rt
}

func (fms *FakeMessageSender) GetState(typ reflect.Type) interface{} {
	st := reflect.New(typ.Elem()).Interface()
	unm, ok := st.(runtime.CBORUnmarshaler)
	require.True(fms.t, ok)
	fms.rt.GetState(unm)
	return st
}

// Send posts a message to the chain
func (fms *FakeMessageSender) Send(ctx context.Context,
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
func (fms *FakeMessageSender) ConstructActor(actorType cid.Cid, newActor, caller address.Address, balance abi.TokenAmount) {
	switch actorType {
	case builtin.InitActorCodeID:
		fms.constructInitActor(newActor)
	case builtin.PaymentChannelActorCodeID:
		fms.constructPaymentChannelActor(newActor, caller, balance)
	}
	fms.rt.Verify()
}

func (fms *FakeMessageSender) ExecAndVerify(caller, receiver address.Address, code cid.Cid, params []byte) {
	a, ok := fms.receivers[receiver]
	if !ok {
		fms.t.Fatalf("actor does not exist %s", receiver.String())
	}
	fms.rt.SetCaller(caller, builtin.AccountActorCodeID)

	a.execAndVerify(fms.t, fms.rt, code, params)
}

// this constructs an initActor harness with the fms mock runtime, so that initActor exports
// can be tested in go-filecoin.
func (fms *FakeMessageSender) constructInitActor(addr address.Address) {
	fms.rt.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	fms.rt.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	h := initActorHarness{}
	ret := fms.rt.Call(h.Constructor, &init_.ConstructorParams{NetworkName: "mock"})
	require.Nil(fms.t, ret)
	fms.receivers[addr] = &h
}

// this constructs a payment channel actor harness with the fms mock runtime, so that paychActor exports
// can be tested in go-filecoin.
//  If you want to test creation of an actor by InitActor use ExecAndVerify
func (fms *FakeMessageSender) constructPaymentChannelActor(newActor, caller address.Address, balance abi.TokenAmount) {

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
