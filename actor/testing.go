package actor

import (
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
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
	"returnRevertError": &exec.FunctionSignature{
		Params: nil,
		Return: nil,
	},
	"goodCall": &exec.FunctionSignature{
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
	return address.Address{}, 0, nil
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
