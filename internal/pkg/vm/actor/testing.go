package actor

import (
	"reflect"

	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

// FakeActorStorage is storage for our fake actor. It contains a single
// bit that is set when the actor's methods are invoked.
type FakeActorStorage struct{ Changed bool }

// FakeActor is a fake actor for use in tests.
type FakeActor struct{}

var _ dispatch.ExecutableActor = (*FakeActor)(nil)

// FakeActor method IDs.
const (
	HasReturnValueID types.MethodID = iota + 32
	ChargeGasAndRevertErrorID
	ReturnRevertErrorID
	goodCallID
	NonZeroExitCodeID
	NestedBalanceID
	sendTokensID
	callSendTokensID
	AttemptMultiSpend1ID
	AttemptMultiSpend2ID
	RunsAnotherMessageID
	BlockLimitTestMethodID
)

var signatures = dispatch.Exports{
	HasReturnValueID: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Address},
	},
	ChargeGasAndRevertErrorID: &dispatch.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	ReturnRevertErrorID: &dispatch.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	goodCallID: &dispatch.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	NonZeroExitCodeID: &dispatch.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	NestedBalanceID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	sendTokensID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	callSendTokensID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	AttemptMultiSpend1ID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	AttemptMultiSpend2ID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	RunsAnotherMessageID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	BlockLimitTestMethodID: &dispatch.FunctionSignature{
		Params: nil,
		Return: nil,
	},
}

// InitializeState stores this actors
func (a *FakeActor) InitializeState(storage runtime.Storage, initializerData interface{}) error {
	st, ok := initializerData.(*FakeActorStorage)
	if !ok {
		return errors.NewFaultError("Initial state to fake actor is not a FakeActorStorage struct")
	}

	stateBytes, err := encoding.Encode(st)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, cid.Undef)
}

// Method returns method definition for a given method id.
func (a *FakeActor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case HasReturnValueID:
		return reflect.ValueOf((*impl)(a).HasReturnValue), signatures[HasReturnValueID], true
	case ChargeGasAndRevertErrorID:
		return reflect.ValueOf((*impl)(a).ChargeGasAndRevertError), signatures[ChargeGasAndRevertErrorID], true
	case ReturnRevertErrorID:
		return reflect.ValueOf((*impl)(a).ReturnRevertError), signatures[ReturnRevertErrorID], true
	case goodCallID:
		return reflect.ValueOf((*impl)(a).GoodCall), signatures[goodCallID], true
	case NonZeroExitCodeID:
		return reflect.ValueOf((*impl)(a).NonZeroExitCode), signatures[NonZeroExitCodeID], true
	case NestedBalanceID:
		return reflect.ValueOf((*impl)(a).NestedBalance), signatures[NestedBalanceID], true
	case sendTokensID:
		return reflect.ValueOf((*impl)(a).SendTokens), signatures[sendTokensID], true
	case callSendTokensID:
		return reflect.ValueOf((*impl)(a).CallSendTokens), signatures[callSendTokensID], true
	case AttemptMultiSpend1ID:
		return reflect.ValueOf((*impl)(a).AttemptMultiSpend1), signatures[AttemptMultiSpend1ID], true
	case AttemptMultiSpend2ID:
		return reflect.ValueOf((*impl)(a).AttemptMultiSpend2), signatures[AttemptMultiSpend2ID], true
	case RunsAnotherMessageID:
		return reflect.ValueOf((*impl)(a).RunsAnotherMessage), signatures[RunsAnotherMessageID], true
	case BlockLimitTestMethodID:
		return reflect.ValueOf((*impl)(a).BlockLimitTestMethod), signatures[BlockLimitTestMethodID], true
	default:
		return nil, nil, false
	}
}

type impl FakeActor

// HasReturnValue is a dummy method that does nothing.
func (*impl) HasReturnValue(ctx runtime.InvocationContext) (address.Address, uint8, error) {
	if err := ctx.Charge(100); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	return address.Undef, 0, nil
}

// ChargeGasAndRevertError simply charges gas and returns a revert error
func (*impl) ChargeGasAndRevertError(ctx runtime.InvocationContext) (uint8, error) {
	if err := ctx.Charge(100); err != nil {
		panic("Unexpected error charging gas")
	}
	return 1, errors.NewRevertError("boom")
}

// ReturnRevertError sets a bit inside fakeActor's storage and returns a
// revert error.
func (*impl) ReturnRevertError(ctx runtime.InvocationContext) (uint8, error) {
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
func (*impl) GoodCall(ctx runtime.InvocationContext) (uint8, error) {
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
func (*impl) NonZeroExitCode(ctx runtime.InvocationContext) (uint8, error) {
	return 42, nil
}

// NestedBalance sends 100 to the given address.
func (*impl) NestedBalance(ctx runtime.InvocationContext, target address.Address) (uint8, error) {
	_, code, err := ctx.Runtime().Send(target, types.SendMethodID, types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// SendTokens sends 100 to the given address.
func (*impl) SendTokens(ctx runtime.InvocationContext, target address.Address) (uint8, error) {
	_, code, err := ctx.Runtime().Send(target, types.SendMethodID, types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// CallSendTokens tells the target to invoke SendTokens to send tokens to the
// to address (that is, it calls target.SendTokens(to)).
func (*impl) CallSendTokens(ctx runtime.InvocationContext, target address.Address, to address.Address) (uint8, error) {
	_, code, err := ctx.Runtime().Send(target, sendTokensID, types.ZeroAttoFIL, []interface{}{to})
	return code, err
}

// AttemptMultiSpend1 attempts to re-spend already spent tokens using a double reentrant call.
func (*impl) AttemptMultiSpend1(ctx runtime.InvocationContext, self, target address.Address) (uint8, error) {
	// This will transfer 100 tokens legitimately.
	_, code, err := ctx.Runtime().Send(target, callSendTokensID, types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed first callSendTokens")
	}
	// Try to double spend
	_, code, err = ctx.Runtime().Send(target, callSendTokensID, types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed second callSendTokens")
	}
	return code, err
}

// AttemptMultiSpend2 attempts to re-spend already spent tokens using a reentrant call followed by a direct spend call.
func (a *impl) AttemptMultiSpend2(ctx runtime.InvocationContext, self, target address.Address) (uint8, error) {
	// This will transfer 100 tokens legitimately.
	_, code, err := ctx.Runtime().Send(target, callSendTokensID, types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed first callSendTokens")
	}
	// Try to triple spend
	code, err = a.SendTokens(ctx, target)
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed sendTokens")
	}
	return code, err
}

// RunsAnotherMessage sends a message
func (*impl) RunsAnotherMessage(ctx runtime.InvocationContext, target address.Address) (uint8, error) {
	if err := ctx.Charge(100); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	_, code, err := ctx.Runtime().Send(target, HasReturnValueID, types.ZeroAttoFIL, []interface{}{})
	return code, err
}

// BlockLimitTestMethod is designed to be used with block gas limit tests. It consumes 1/4 of the
// block gas limit per run. Please ensure message.gasLimit >= 1/4 of block limit or it will panic.
func (*impl) BlockLimitTestMethod(ctx runtime.InvocationContext) (uint8, error) {
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
