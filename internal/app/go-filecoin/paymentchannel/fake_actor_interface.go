package paymentchannel

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/support/mock"
	spect "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// FakeActorInterface fulfils the MsgSender and MsgWaiter interfaces for a Manager
// via the specs_actors mock runtime. It executes init.Actor exports directly.
type FakeActorInterface struct {
	t   *testing.T
	ctx context.Context
	*mock.Runtime
	*initActorHarness
	newActor, newActorID, caller address.Address
	result                       MsgResult
}

// NewFakeActorInterface initializes a FakeActorInterface and constructs
// the InitActor.
func NewFakeActorInterface(t *testing.T, ctx context.Context, rtbuilder *mock.RuntimeBuilder) *FakeActorInterface {
	fms := &FakeActorInterface{
		ctx:              ctx,
		Runtime:          rtbuilder.Build(t),
		initActorHarness: new(initActorHarness),
		t:                t,
	}
	fms.constructInitActor()
	fms.Runtime.Verify()
	return fms
}

// Send simulates posting to chain but calls actor code directly
func (fms *FakeActorInterface) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	execParams, ok := params.(init_.ExecParams)
	require.True(fms.t, ok)
	if method == builtin.MethodsInit.Exec {
		fms.ExecAndVerify(from, value, &execParams)
	}

	return fms.result.MsgCid, nil, nil
}

// Wait simulates waiting for the result of a message and calls the callback `cb`
func (fms *FakeActorInterface) Wait(_ context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	require.Equal(fms.t, msgCid, fms.result.MsgCid)
	res := fms.result
	return cb(res.Block, res.Msg, res.Rcpt)
}

// StubCtorSendResponse sets up addresses for the initActor and generates
// message responses from the call to create a new payment channel
func (fms *FakeActorInterface) StubCtorSendResponse(msgVal abi.TokenAmount) (msgCid cid.Cid, client, miner, idaddr, uniqueaddr address.Address) {
	fms.caller = spect.NewActorAddr(fms.t, "client account addr")
	fms.newActor = spect.NewActorAddr(fms.t, "new paych actor addr")
	fms.newActorID = spect.NewIDAddr(fms.t, 100)
	miner = spect.NewActorAddr(fms.t, "miner account addr")
	fms.Runtime.SetNewActorAddress(fms.newActor)

	msgCid, msgRes := GenCreatePaychActorMessage(fms.t, fms.caller, miner, fms.newActor, msgVal, exitcode.Ok, 42)
	fms.result = msgRes
	return msgCid, fms.caller, miner, fms.newActorID, fms.newActor
}

// ExecAndVerify sets up
func (fms *FakeActorInterface) ExecAndVerify(caller address.Address, value abi.TokenAmount, params *init_.ExecParams) {
	a := fms.initActorHarness
	expParams := runtime.CBORBytes(params.ConstructorParams)

	fms.Runtime.SetReceived(value)
	fms.Runtime.SetCaller(caller, builtin.AccountActorCodeID)
	fms.Runtime.ExpectCreateActor(builtin.PaymentChannelActorCodeID, fms.newActorID)

	fms.Runtime.ExpectSend(fms.newActorID, builtin.MethodConstructor, expParams, value, nil, exitcode.Ok)
	exret := a.execAndVerify(fms.t, fms.Runtime, params)
	require.Equal(fms.t, fms.newActor, exret.RobustAddress)
	require.Equal(fms.t, fms.newActorID, exret.IDAddress)
}

// constructInitActor constructs an initActor harness with the fms mock runtime, so that initActor exports
// can be tested in go-filecoin.
func (fms *FakeActorInterface) constructInitActor() {
	fms.Runtime.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	fms.Runtime.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	h := &initActorHarness{}
	ret := fms.Runtime.Call(h.Constructor, &init_.ConstructorParams{NetworkName: "mock"})
	require.Nil(fms.t, ret)
	fms.initActorHarness = h
}

// actor harnesses should be very lightweight.
type initActorHarness struct {
	init_.Actor
	t testing.TB
}

func (h *initActorHarness) execAndVerify(t *testing.T, rt *mock.Runtime, params *init_.ExecParams) *init_.ExecReturn {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.Exec, params).(*init_.ExecReturn)
	require.NotNil(t, ret)
	rt.Verify()
	return ret
}
