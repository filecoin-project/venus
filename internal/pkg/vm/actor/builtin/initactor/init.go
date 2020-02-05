package initactor

import (
	"context"
	"math/big"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

// Actor is the builtin actor responsible for network initialization.
// More details on future responsibilities can be found at https://github.com/filecoin-project/specs/blob/master/actors.md#init-actor.
type Actor struct{}

// State is the init actor's storage.
type State struct {
	Network string
	// Address -> ActorID
	AddressMap cid.Cid `refmt:",omitempty"`
	IDMap      cid.Cid `refmt:",omitempty"`
	NextID     types.Uint64
}

// View is a readonly view into the actor state
type View struct {
	state State
	store runtime.Storage
}

// assignNewID returns the nextID and increments the counter
func (s *State) assignNewID() types.Uint64 {
	id := s.NextID
	s.NextID++
	return id
}

// Actor methods
const (
	ExecMethodID                 types.MethodID = 3
	GetActorIDForAddressMethodID types.MethodID = 4
	// currently unspecified
	GetAddressForActorIDMethodID types.MethodID = iota + 32
	GetNetworkMethodID
)

// NewActor returns a init actor.
func NewActor() *actor.Actor {
	return actor.NewActor(types.InitActorCodeCid, types.ZeroAttoFIL)
}

// NewView creates a new init actor state view.
func NewView(stateHandle runtime.ReadonlyActorStateHandle, store runtime.Storage) View {
	// load state as readonly
	var state State
	stateHandle.Readonly(&state)
	// return view
	return View{
		state: state,
		store: store,
	}
}

// GetIDAddressByAddress returns the IDAddress for the target address.
//
// If the lookup fails, this method will return false.
func (v *View) GetIDAddressByAddress(target address.Address) (address.Address, bool) {
	if target.Protocol() == address.ID {
		return target, true
	}
	lookup, err := actor.LoadLookup(context.Background(), v.store, v.state.AddressMap)
	if err != nil {
		panic("could not load internal state")
	}

	var id types.Uint64
	err = lookup.Find(context.Background(), target.String(), &id)
	if err != nil {
		if err == hamt.ErrNotFound {
			return address.Undef, false
		}
		panic("could not load internal state")
	}

	idAddr, err := address.NewIDAddress((uint64)(id))
	if err != nil {
		panic("could not load internal state")
	}
	return idAddr, true
}

//
// ExecutableActor impl for Actor
//

// Ensure InitActor is an ExecutableActor at compile time.
var _ dispatch.ExecutableActor = (*Actor)(nil)

var signatures = dispatch.Exports{
	GetNetworkMethodID: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.String},
	},
	ExecMethodID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Cid, abi.Parameters},
		Return: []abi.Type{abi.Address},
	},
	GetActorIDForAddressMethodID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{abi.Integer},
	},
	GetAddressForActorIDMethodID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: []abi.Type{abi.Address},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case ExecMethodID:
		return reflect.ValueOf((*Impl)(a).Exec), signatures[ExecMethodID], true
	case GetActorIDForAddressMethodID:
		return reflect.ValueOf((*Impl)(a).GetActorIDForAddress), signatures[GetActorIDForAddressMethodID], true
	case GetAddressForActorIDMethodID:
		return reflect.ValueOf((*Impl)(a).GetAddressForActorID), signatures[GetAddressForActorIDMethodID], true
	case GetNetworkMethodID:
		return reflect.ValueOf((*Impl)(a).GetNetwork), signatures[GetNetworkMethodID], true
	default:
		return nil, nil, false
	}
}

// InitializeState for init actor.
func (*Actor) InitializeState(handle runtime.ActorStateHandle, params interface{}) error {
	network, ok := params.(string)
	if !ok {
		return errors.NewRevertError("init actor network parameter is not a string")
	}

	initStorage := &State{
		Network: network,
		NextID:  100,
	}
	handle.Create(initStorage)

	return nil
}

//
// public methods for actor
//

// LookupIDAddress returns the the ActorID for a given address.
func LookupIDAddress(rt runtime.InvocationContext, addr address.Address) (uint64, bool, error) {
	var state State
	id, err := rt.StateHandle().Transaction(&state, func() (interface{}, error) {
		return lookupIDAddress(rt, state, addr)
	})
	if err != nil {
		if err == hamt.ErrNotFound {
			return 0, false, nil
		}
		return 0, false, errors.FaultErrorWrap(err, "could not lookup actor id")
	}

	return uint64(id.(types.Uint64)), true, nil
}

//
// vm methods for actor
//

const (
	// ErrNotFound indicates an attempt to lookup a nonexistant address
	ErrNotFound = 32
)

// invocationContext is the context for the init actor.
type invocationContext interface {
	runtime.InvocationContext
	CreateActor(actorID types.Uint64, code cid.Cid, params []interface{}) (address.Address, address.Address)
}

// Impl is the VM implementation of the actor.
type Impl Actor

// GetNetwork returns the network name for this network
func (*Impl) GetNetwork(ctx runtime.InvocationContext) (string, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return "", internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ctx.StateHandle().Readonly(&state)
	return state.Network, 0, nil
}

// GetActorIDForAddress looks up the actor id for a filecoin address.
func (a *Impl) GetActorIDForAddress(ctx invocationContext, addr address.Address) (*big.Int, uint8, error) {
	ctx.ValidateCaller(pattern.Any{})

	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return big.NewInt(0), internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	id, err := ctx.StateHandle().Transaction(&state, func() (interface{}, error) {
		return lookupIDAddress(ctx, state, addr)
	})
	if err != nil {
		if err == hamt.ErrNotFound {
			return nil, ErrNotFound, errors.NewCodedRevertErrorf(ErrNotFound, "actor id not found for address: %s", addr)
		}
		return nil, errors.CodeError(err), errors.FaultErrorWrap(err, "could not lookup actor id")
	}

	return big.NewInt(int64(id.(types.Uint64))), 0, nil
}

// GetAddressForActorID looks up the address for an actor id.
func (a *Impl) GetAddressForActorID(vmctx runtime.InvocationContext, actorID types.Uint64) (address.Address, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	vmctx.StateHandle().Readonly(state)

	ctx := context.TODO()
	lookup, err := actor.LoadLookup(ctx, vmctx.Runtime().Storage(), state.IDMap)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", state.IDMap)
	}

	key, err := keyForActorID(actorID)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrapf(err, "could not encode actor id: %d", actorID)
	}

	var addr address.Address
	err = lookup.Find(ctx, key, &addr)
	if err != nil {
		if err == hamt.ErrNotFound {
			return address.Undef, ErrNotFound, errors.NewCodedRevertErrorf(ErrNotFound, "actor address not found for id: %d", actorID)
		}
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrap(err, "could not lookup actor address")
	}

	return addr, 0, nil
}

// Exec creates a new builtin actor.
func (a *Impl) Exec(vmctx invocationContext, codeCID cid.Cid, params []interface{}) (address.Address, uint8, error) {
	vmctx.ValidateCaller(pattern.Any{})

	// Dragons: clean this up to match spec
	var state State
	out, err := vmctx.StateHandle().Transaction(&state, func() (interface{}, error) {
		// create id address
		actorID := state.assignNewID()

		return actorID, nil
	})
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	actorID := out.(types.Uint64)

	actorIdAddr, actorAddr := vmctx.CreateActor(actorID, codeCID, params)

	_, err = vmctx.StateHandle().Transaction(&state, func() (interface{}, error) {
		var err error

		// map id to address and vice versa
		ctx := context.TODO()
		state.AddressMap, err = setID(ctx, vmctx.Runtime().Storage(), state.AddressMap, actorAddr, actorID)
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not save id by address")
		}

		state.IDMap, err = setAddress(ctx, vmctx.Runtime().Storage(), state.IDMap, actorID, actorAddr)
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not save address by id")
		}

		return nil, nil
	})
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	// Dragons: the idaddress is returned by the spec
	return actorIdAddr, 0, nil
}

func lookupIDAddress(vmctx runtime.InvocationContext, state State, addr address.Address) (types.Uint64, error) {
	ctx := context.TODO()
	lookup, err := actor.LoadLookup(ctx, vmctx.Runtime().Storage(), state.AddressMap)
	if err != nil {
		return 0, errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", state.IDMap)
	}

	var id types.Uint64
	err = lookup.Find(ctx, addr.String(), &id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func setAddress(ctx context.Context, storage runtime.Storage, idMap cid.Cid, actorID types.Uint64, addr address.Address) (cid.Cid, error) {
	lookup, err := actor.LoadLookup(ctx, storage, idMap)
	if err != nil {
		return cid.Undef, errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", idMap)
	}

	key, err := keyForActorID(actorID)
	if err != nil {
		return cid.Undef, err
	}

	err = lookup.Set(ctx, key, addr)
	if err != nil {
		return cid.Undef, errors.FaultErrorWrapf(err, "could not set address")
	}

	return lookup.Commit(ctx)
}

func setID(ctx context.Context, storage runtime.Storage, addressMap cid.Cid, addr address.Address, actorID types.Uint64) (cid.Cid, error) {
	lookup, err := actor.LoadLookup(ctx, storage, addressMap)
	if err != nil {
		return cid.Undef, errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", addressMap)
	}

	err = lookup.Set(ctx, addr.String(), actorID)
	if err != nil {
		return cid.Undef, errors.FaultErrorWrapf(err, "could not set id")
	}

	return lookup.Commit(ctx)
}

func keyForActorID(actorID types.Uint64) (string, error) {
	key, err := encoding.Encode(actorID)
	if err != nil {
		return "", errors.FaultErrorWrapf(err, "could not encode actor id: %d", actorID)
	}

	return string(key), nil
}

func (a *Impl) isBuiltinActor(code cid.Cid) bool {
	return code.Equals(types.StorageMarketActorCodeCid) ||
		code.Equals(types.InitActorCodeCid) ||
		code.Equals(types.MinerActorCodeCid) ||
		code.Equals(types.BootstrapMinerActorCodeCid)
}

func (a *Impl) isSingletonActor(code cid.Cid) bool {
	return code.Equals(types.StorageMarketActorCodeCid) ||
		code.Equals(types.InitActorCodeCid)
}
