package actor

import (
	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
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
	"returnRevertError": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	"goodCall": &exec.FunctionSignature{
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

	return storage.Commit(id, nil)
}

// Exports returns the list of fake actor exported functions.
func (ma *FakeActor) Exports() exec.Exports {
	return FakeActorExports
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

// NestedBalance sents 100 to the given address.
func (ma *FakeActor) NestedBalance(ctx exec.VMContext, target types.Address) (uint8, error) {
	_, code, err := ctx.Send(target, "", types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// SendTokens sends 100 to the given address.
func (ma *FakeActor) SendTokens(ctx exec.VMContext, target types.Address) (uint8, error) {
	_, code, err := ctx.Send(target, "", types.NewAttoFILFromFIL(100), nil)
	return code, err
}

// CallSendTokens tells the target to invoke SendTokens to send tokens to the
// to address (that is, it calls target.SendTokens(to)).
func (ma *FakeActor) CallSendTokens(ctx exec.VMContext, target types.Address, to types.Address) (uint8, error) {
	_, code, err := ctx.Send(target, "sendTokens", types.ZeroAttoFIL, []interface{}{to})
	return code, err
}

// AttemptMultiSpend1 attempts to re-spend already spent tokens using a double reentrant call.
func (ma *FakeActor) AttemptMultiSpend1(ctx exec.VMContext, self, target types.Address) (uint8, error) {
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
func (ma *FakeActor) AttemptMultiSpend2(ctx exec.VMContext, self, target types.Address) (uint8, error) {
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
