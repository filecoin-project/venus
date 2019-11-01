package actor

import (
	"reflect"

	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vladrok"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vladrok/kungfu"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vladrok/pandas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
)

// FakeActorStorage is storage for our fake actor. It contains a single
// bit that is set when the actor's methods are invoked.
type FakeActorStorage struct{ Changed bool }

// FakeActor is a fake actor for use in tests.
type FakeActor struct{}

var _ kungfu.ExecutableActor = (*FakeActor)(nil)

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

var signatures = kungfu.Exports{
	HasReturnValueID: &pandas.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.Address},
	},
	ChargeGasAndRevertErrorID: &pandas.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	ReturnRevertErrorID: &pandas.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	goodCallID: &pandas.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	NonZeroExitCodeID: &pandas.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	NestedBalanceID: &pandas.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	sendTokensID: &pandas.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	callSendTokensID: &pandas.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	AttemptMultiSpend1ID: &pandas.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	AttemptMultiSpend2ID: &pandas.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address},
		Return: nil,
	},
	RunsAnotherMessageID: &pandas.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	BlockLimitTestMethodID: &pandas.FunctionSignature{
		Params: nil,
		Return: nil,
	},
}

// InitializeState stores this actors
func (a *FakeActor) InitializeState(storage vladrok.Storage, initializerData interface{}) error {
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
func (a *FakeActor) Method(id types.MethodID) (kungfu.Method, *pandas.FunctionSignature, bool) {
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
func (*impl) HasReturnValue(ctx vladrok.Runtime) (address.Address, uint8, error) {
	if err := ctx.Charge(100); err != nil {
		return address.Undef, kungfu.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	return address.Undef, 0, nil
}

// ChargeGasAndRevertError simply charges gas and returns a revert error
func (*impl) ChargeGasAndRevertError(ctx vladrok.Runtime) (uint8, error) {
	if err := ctx.Charge(100); err != nil {
		panic("Unexpected error charging gas")
	}
	return 1, errors.NewRevertError("boom")
}

// ReturnRevertError sets a bit inside fakeActor's storage and returns a
// revert error.
func (*impl) ReturnRevertError(ctx vladrok.Runtime) (uint8, error) {
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
func (*impl) GoodCall(ctx vladrok.Runtime) (uint8, error) {
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
func (*impl) NonZeroExitCode(ctx vladrok.Runtime) (uint8, error) {
	return 42, nil
}

// NestedBalance sends 100 to the given address.
func (*impl) NestedBalance(ctx vladrok.Runtime, target address.Address) (uint8, error) {
	_, code, err := ctx.Send(target, types.SendMethodID, types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// SendTokens sends 100 to the given address.
func (*impl) SendTokens(ctx vladrok.Runtime, target address.Address) (uint8, error) {
	_, code, err := ctx.Send(target, types.SendMethodID, types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// CallSendTokens tells the target to invoke SendTokens to send tokens to the
// to address (that is, it calls target.SendTokens(to)).
func (*impl) CallSendTokens(ctx vladrok.Runtime, target address.Address, to address.Address) (uint8, error) {
	_, code, err := ctx.Send(target, sendTokensID, types.ZeroAttoFIL, []interface{}{to})
	return code, err
}

// AttemptMultiSpend1 attempts to re-spend already spent tokens using a double reentrant call.
func (*impl) AttemptMultiSpend1(ctx vladrok.Runtime, self, target address.Address) (uint8, error) {
	// This will transfer 100 tokens legitimately.
	_, code, err := ctx.Send(target, callSendTokensID, types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed first callSendTokens")
	}
	// Try to double spend
	_, code, err = ctx.Send(target, callSendTokensID, types.ZeroAttoFIL, []interface{}{self, target})
	if code != 0 || err != nil {
		return code, errors.FaultErrorWrap(err, "failed second callSendTokens")
	}
	return code, err
}

// AttemptMultiSpend2 attempts to re-spend already spent tokens using a reentrant call followed by a direct spend call.
func (a *impl) AttemptMultiSpend2(ctx vladrok.Runtime, self, target address.Address) (uint8, error) {
	// This will transfer 100 tokens legitimately.
	_, code, err := ctx.Send(target, callSendTokensID, types.ZeroAttoFIL, []interface{}{self, target})
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
func (*impl) RunsAnotherMessage(ctx vladrok.Runtime, target address.Address) (uint8, error) {
	if err := ctx.Charge(100); err != nil {
		return kungfu.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	_, code, err := ctx.Send(target, HasReturnValueID, types.ZeroAttoFIL, []interface{}{})
	return code, err
}

// BlockLimitTestMethod is designed to be used with block gas limit tests. It consumes 1/4 of the
// block gas limit per run. Please ensure message.gasLimit >= 1/4 of block limit or it will panic.
func (*impl) BlockLimitTestMethod(ctx vladrok.Runtime) (uint8, error) {
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
