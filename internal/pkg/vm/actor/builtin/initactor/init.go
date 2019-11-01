package initactor

import (
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/vminternal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/external"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
)

// Actor is the builtin actor responsible for network initialization.
// More details on future responsibilities can be found at https://github.com/filecoin-project/specs/blob/master/actors.md#init-actor.
type Actor struct{}

// State is the init actor's storage.
type State struct {
	Network string
}

// Actor methods
const (
	GetNetwork types.MethodID = iota + 32
)

// NewActor returns a init actor.
func NewActor() *actor.Actor {
	return actor.NewActor(types.InitActorCodeCid, types.ZeroAttoFIL)
}

//
// ExecutableActor impl for Actor
//

// Ensure InitActor is an ExecutableActor at compile time.
var _ vminternal.ExecutableActor = (*Actor)(nil)

var signatures = vminternal.Exports{
	GetNetwork: &external.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.String},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (vminternal.Method, *external.FunctionSignature, bool) {
	switch id {
	case GetNetwork:
		return reflect.ValueOf((*Impl)(a).GetNetwork), signatures[GetNetwork], true
	default:
		return nil, nil, false
	}
}

// InitializeState for init actor.
func (*Actor) InitializeState(storage vm2.Storage, networkInterface interface{}) error {
	network := networkInterface.(string)

	initStorage := &State{
		Network: network,
	}
	stateBytes, err := encoding.Encode(initStorage)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, cid.Undef)
}

//
// vm methods for actor
//

// Impl is the VM implementation of the actor.
type Impl Actor

// GetNetwork returns the network name for this network
func (*Impl) GetNetwork(ctx vm2.Runtime) (string, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return "", vminternal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	err := actor.ReadState(ctx, &state)
	if err != nil {
		return "", errors.CodeError(err), err
	}

	return state.Network, 0, nil
}
