package actor

import (
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

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
}

// Exports returns the list of fake actor exported functions.
func (ma *FakeActor) Exports() exec.Exports {
	return FakeActorExports
}

// ReturnRevertError sets a bit inside fakeActor's storage and returns a
// revert error.
func (ma *FakeActor) ReturnRevertError(ctx exec.VMContext) (uint8, error) {
	fastore := &FakeActorStorage{}
	_, err := WithStorage(ctx, fastore, func() (interface{}, error) {
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
	_, err := WithStorage(ctx, fastore, func() (interface{}, error) {
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
	_, code, err := ctx.Send(target, "", types.NewTokenAmount(100), nil)
	return code, err
}

// NewStorage returns an empty FakeActorStorage struct
func (ma *FakeActor) NewStorage() interface{} {
	return &FakeActorStorage{}
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
