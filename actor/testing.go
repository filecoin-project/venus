package actor

import (
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

func init() {
	cbor.RegisterCborType(FakeActorStorage{})
}

// FakeActorStorage is storage for our fake actor. It contains a single
// bit that is set when the actor's methods are invoked.
type FakeActorStorage struct{ Changed bool }

// FakeActor is a fake actor for use in tests.
type FakeActor struct{}

var _ exec.ExecutableActor = (*FakeActor)(nil)

// FakeActorExports are the exports of the fake actor.
var FakeActorExports = exec.Exports{
	"hasReturnValue": &exec.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Address},
	},
	"chargeGasAndRevertError": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	"returnRevertError": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	"goodCall": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	"nonZeroExitCode": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	"nestedBalance": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	"sendTokens": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	"callSendTokens": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	"attemptMultiSpend1": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	"attemptMultiSpend2": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	"runsAnotherMessage": &exec.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	"blockLimitTestMethod": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
}

// InitializeState stores this actors
func (ma *FakeActor) InitializeState(storage exec.Storage, initializerData interface{}) error {
	st, ok := initializerData.(*FakeActorStorage)
	if !ok {
		return errors.NewFaultError("Initial state to fake actor is not a FakeActorStorage struct")
	}

	stateBytes, err := cbor.DumpObject(st)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, cid.Undef)
}

// Exports returns the list of fake actor exported functions.
func (ma *FakeActor) Exports() exec.Exports {
	return FakeActorExports
}

// HasReturnValue is a dummy method that does nothing.
func (ma *FakeActor) HasReturnValue(ctx exec.VMContext) (address.Address, uint8, error) {
	if err := ctx.Charge(100); err != nil {
		return address.Undef, exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	return address.Undef, 0, nil
}

// ChargeGasAndRevertError simply charges gas and returns a revert error
func (ma *FakeActor) ChargeGasAndRevertError(ctx exec.VMContext) (uint8, error) {
	if err := ctx.Charge(100); err != nil {
		panic("Unexpected error charging gas")
	}
	return 1, errors.NewRevertError("boom")
}

// ReturnRevertError sets a bit inside fakeActor's storage and returns a
// revert error.
func (ma *FakeActor) ReturnRevertError(ctx exec.VMContext) (uint8, error) {
	fastore := &FakeActorStorage{}
	_, err := WithState(ctx, fastore, func() (interface{}, error) {
		fastore.Changed = true
		return nil, nil
	})
	if err != nil {
		panic(err.Error())
	}
	return 1, errors.NewRevertError("boom")
}

// GoodCall sets a bit inside fakeActor's storage.
func (ma *FakeActor) GoodCall(ctx exec.VMContext) (uint8, error) {
	fastore := &FakeActorStorage{}
	_, err := WithState(ctx, fastore, func() (interface{}, error) {
		fastore.Changed = true
		return nil, nil
	})
	if err != nil {
		panic(err.Error())
	}
	return 0, nil
}

// NonZeroExitCode returns a nonzero exit code but no error.
func (ma *FakeActor) NonZeroExitCode(ctx exec.VMContext) (uint8, error) {
	return 42, nil
}

// NestedBalance sents 100 to the given address.
func (ma *FakeActor) NestedBalance(ctx exec.VMContext, target address.Address) (uint8, error) {
	_, code, err := ctx.Send(target, "", types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// SendTokens sends 100 to the given address.
func (ma *FakeActor) SendTokens(ctx exec.VMContext, target address.Address) (uint8, error) {
	_, code, err := ctx.Send(target, "", types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// CallSendTokens tells the target to invoke SendTokens to send tokens to the
// to address (that is, it calls target.SendTokens(to)).
func (ma *FakeActor) CallSendTokens(ctx exec.VMContext, target address.Address, to address.Address) (uint8, error) {
	_, code, err := ctx.Send(target, "sendTokens", types.ZeroAttoFIL, []interface{}{to})
	return code, err
}

// AttemptMultiSpend1 attempts to re-spend already spent tokens using a double reentrant call.
func (ma *FakeActor) AttemptMultiSpend1(ctx exec.VMContext, self, target address.Address) (uint8, error) {
	// This will transfer 100 tokens legitimately.
	_, code, err := ctx.Send(target, "callSendTokens", types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed first callSendTokens")
	}
	// Try to double spend
	_, code, err = ctx.Send(target, "callSendTokens", types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed second callSendTokens")
	}
	return code, err
}

// AttemptMultiSpend2 attempts to re-spend already spent tokens using a reentrant call followed by a direct spend call.
func (ma *FakeActor) AttemptMultiSpend2(ctx exec.VMContext, self, target address.Address) (uint8, error) {
	// This will transfer 100 tokens legitimately.
	_, code, err := ctx.Send(target, "callSendTokens", types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed first callSendTokens")
	}
	// Try to triple spend
	code, err = ma.SendTokens(ctx, target)
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed sendTokens")
	}
	return code, err
}

// RunsAnotherMessage sends a message
func (ma *FakeActor) RunsAnotherMessage(ctx exec.VMContext, target address.Address) (uint8, error) {
	if err := ctx.Charge(100); err != nil {
		return exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	_, code, err := ctx.Send(target, "hasReturnValue", types.ZeroAttoFIL, []interface{}{})
	return code, err
}

// BlockLimitTestMethod is designed to be used with block gas limit tests. It consumes 1/4 of the
// block gas limit per run. Please ensure message.gasLimit >= 1/4 of block limit or it will panic.
func (ma *FakeActor) BlockLimitTestMethod(ctx exec.VMContext) (uint8, error) {
	if err := ctx.Charge(types.BlockGasLimit / 4); err != nil {
		panic("designed for block limit testing, ensure msg limit is adequate")
	}
	return 0, nil
}

// MustConvertParams encodes the given params and panics if it fails to do so.
func MustConvertParams(params ...interface{}) []byte {
	vals, err := abi.ToValues(params)
	if err != nil {
		panic(err)
	}

	out, err := abi.EncodeValues(vals)
	if err != nil {
		panic(err)
	}
	return out
}
