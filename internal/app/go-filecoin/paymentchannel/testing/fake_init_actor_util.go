package testing

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

// FakeInitActorUtil fulfils the MsgSender and MsgWaiter interfaces for a Manager
// via the specs_actors mock runtime. It executes init.Actor exports directly.
type FakeInitActorUtil struct {
	t   *testing.T
	ctx context.Context
	*mock.Runtime
	*initActorHarness
	newActor, newActorID, caller address.Address
	result                       MsgResult
	msgSender                    sender
	msgWaiter                    waiter
}

// NewFakeInitActorUtil initializes a FakeInitActorUtil and constructs
// the InitActor.
func NewFakeInitActorUtil(ctx context.Context, t *testing.T, balance abi.TokenAmount) *FakeInitActorUtil {

	builder := mock.NewBuilder(context.Background(), builtin.InitActorAddr).
		WithBalance(balance, abi.NewTokenAmount(0))

	fai := &FakeInitActorUtil{
		ctx:              ctx,
		Runtime:          builder.Build(t),
		initActorHarness: new(initActorHarness),
		t:                t,
	}
	fai.constructInitActor()
	fai.Runtime.Verify()
	fai.msgSender = fai.defaultSend
	fai.msgWaiter = fai.defaultWait
	return fai
}

type sender func(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error)

type waiter func(_ context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error

// Send simulates posting to chain but calls actor code directly
func (fai *FakeInitActorUtil) Send(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {

	return fai.msgSender(ctx, from, to, value, gasPrice, gasLimit, bcast, method, params)
}
func (fai *FakeInitActorUtil) defaultSend(ctx context.Context,
	from, to address.Address,
	value types.AttoFIL,
	gasPrice types.AttoFIL,
	gasLimit gas.Unit,
	bcast bool,
	method abi.MethodNum,
	params interface{}) (out cid.Cid, pubErrCh chan error, err error) {
	execParams, ok := params.(*init_.ExecParams)
	require.True(fai.t, ok)
	fai.ExecAndVerify(from, value, execParams)
	return fai.result.MsgCid, nil, nil
}

// Wait simulates waiting for the result of a message and calls the callback `cb`
func (fai *FakeInitActorUtil) Wait(ctx context.Context, msgCid cid.Cid, lookback uint64, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	return fai.msgWaiter(ctx, msgCid, cb)
}

func (fai *FakeInitActorUtil) defaultWait(_ context.Context, msgCid cid.Cid, cb func(*block.Block, *types.SignedMessage, *vm.MessageReceipt) error) error {
	require.Equal(fai.t, msgCid, fai.result.MsgCid)
	res := fai.result
	return cb(res.Block, res.Msg, res.Rcpt)
}

// DelegateSender allows test to delegate a sender function
func (fai *FakeInitActorUtil) DelegateSender(delegate sender) {
	fai.msgSender = delegate
}

// DelegateWaiter allows test to deletate a waiter function
func (fai *FakeInitActorUtil) DelegateWaiter(delegate waiter) {
	fai.msgWaiter = delegate
}

// StubCtorSendResponse sets up addresses for the initActor and generates
// message responses from the call to create a new payment channel
func (fai *FakeInitActorUtil) StubCtorSendResponse(msgVal abi.TokenAmount) (msgCid cid.Cid, client, miner, idaddr, uniqueaddr address.Address) {
	fai.caller = spect.NewActorAddr(fai.t, "client account addr")
	fai.newActor = spect.NewActorAddr(fai.t, "new paych actor addr")
	fai.newActorID = spect.NewIDAddr(fai.t, 100)
	miner = spect.NewActorAddr(fai.t, "miner account addr")
	fai.Runtime.SetNewActorAddress(fai.newActor)

	msgCid, msgRes := GenCreatePaychActorMessage(fai.t, fai.caller, miner, fai.newActor, msgVal, exitcode.Ok, 42)
	fai.result = msgRes
	return msgCid, fai.caller, miner, fai.newActorID, fai.newActor
}

// ExecAndVerify sets up init actor to execute a constructor given a caller and value
func (fai *FakeInitActorUtil) ExecAndVerify(caller address.Address, value abi.TokenAmount, params *init_.ExecParams) {
	a := fai.initActorHarness
	expParams := runtime.CBORBytes(params.ConstructorParams)

	fai.Runtime.SetReceived(value)
	fai.Runtime.SetCaller(caller, builtin.AccountActorCodeID)
	fai.Runtime.ExpectCreateActor(builtin.PaymentChannelActorCodeID, fai.newActorID)

	fai.Runtime.ExpectSend(fai.newActorID, builtin.MethodConstructor, expParams, value, nil, exitcode.Ok)
	exret := a.execAndVerify(fai.Runtime, params)
	require.Equal(fai.t, fai.newActor, exret.RobustAddress)
	require.Equal(fai.t, fai.newActorID, exret.IDAddress)
}

// constructInitActor constructs an initActor harness with the fai mock runtime, so that initActor exports
// can be tested in go-filecoin.
func (fai *FakeInitActorUtil) constructInitActor() {
	fai.Runtime.SetCaller(builtin.SystemActorAddr, builtin.SystemActorCodeID)
	fai.Runtime.ExpectValidateCallerAddr(builtin.SystemActorAddr)
	h := &initActorHarness{}
	ret := fai.Runtime.Call(h.Constructor, &init_.ConstructorParams{NetworkName: "mock"})
	require.Nil(fai.t, ret)
	fai.initActorHarness = h
}

// actor harnesses should be very lightweight.
type initActorHarness struct {
	init_.Actor
	t testing.TB
}

func (h *initActorHarness) execAndVerify(rt *mock.Runtime, params *init_.ExecParams) *init_.ExecReturn {
	rt.ExpectValidateCallerAny()
	ret := rt.Call(h.Exec, params).(*init_.ExecReturn)
	require.NotNil(h.t, ret)
	rt.Verify()
	return ret
}
