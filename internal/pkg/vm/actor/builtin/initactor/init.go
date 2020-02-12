package initactor

import (
	"context"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
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
	return actor.NewActor(types.InitActorCodeCid, abi.NewTokenAmount(0))
}

// NewView creates a new init actor state view.
func NewView(stateHandle runtime.ActorStateHandle, store runtime.Storage) View {
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
	err = lookup.Find(context.Background(), string(target.Bytes()), &id)
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
var _ dispatch.Actor = (*Actor)(nil)

// Exports implements `dispatch.Actor`
func (a *Actor) Exports() []interface{} {
	return []interface{}{
		ExecMethodID:                 (*Impl)(a).Exec,
		GetActorIDForAddressMethodID: (*Impl)(a).GetActorIDForAddress,
	}
}

// InitializeState for init actor.
func (*Actor) InitializeState(handle runtime.ActorStateHandle, params interface{}) error {
	network, ok := params.(string)
	if !ok {
		return fmt.Errorf("init actor network parameter is not a string")
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
	id, err := rt.State().Transaction(&state, func() (interface{}, error) {
		return lookupIDAddress(rt, state, addr)
	})
	if err != nil {
		if err == hamt.ErrNotFound {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("could not lookup actor id")
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
	CreateActor(actorID types.Uint64, code cid.Cid, params []byte) (address.Address, address.Address)
}

// Impl is the VM implementation of the actor.
type Impl Actor

// GetNetwork returns the network name for this network
func (*Impl) GetNetwork(ctx runtime.InvocationContext) (string, uint8, error) {
	var state State
	ctx.State().Readonly(&state)
	return state.Network, 0, nil
}

// GetActorIDForAddress looks up the actor id for a filecoin address.
func (a *Impl) GetActorIDForAddress(ctx invocationContext, addr address.Address) (*big.Int, uint8, error) {
	ctx.ValidateCaller(pattern.Any{})

	var state State
	id, err := ctx.State().Transaction(&state, func() (interface{}, error) {
		return lookupIDAddress(ctx, state, addr)
	})
	if err != nil {
		if err == hamt.ErrNotFound {
			return nil, ErrNotFound, fmt.Errorf("actor id not found for address: %s", addr)
		}
		return nil, 1, fmt.Errorf("could not lookup actor id")
	}

	return big.NewInt(int64(id.(types.Uint64))), 0, nil
}

// GetAddressForActorID looks up the address for an actor id.
func (a *Impl) GetAddressForActorID(vmctx runtime.InvocationContext, actorID types.Uint64) (address.Address, uint8, error) {
	var state State
	vmctx.State().Readonly(state)

	ctx := context.TODO()
	lookup, err := actor.LoadLookup(ctx, vmctx.Runtime().Storage(), state.IDMap)
	if err != nil {
		return address.Undef, 1, fmt.Errorf("could not load lookup for cid: %s", state.IDMap)
	}

	key, err := keyForActorID(actorID)
	if err != nil {
		return address.Undef, 1, fmt.Errorf("could not encode actor id: %d", actorID)
	}

	var addr address.Address
	err = lookup.Find(ctx, key, &addr)
	if err != nil {
		if err == hamt.ErrNotFound {
			return address.Undef, ErrNotFound, fmt.Errorf("actor address not found for id: %d", actorID)
		}
		return address.Undef, 1, fmt.Errorf("could not lookup actor address")
	}

	return addr, 0, nil
}

// ExecParams are the params for the Exec method.
type ExecParams struct {
	ActorCodeCid      cid.Cid
	ConstructorParams []byte
}

// Exec creates a new builtin actor.
func (a *Impl) Exec(vmctx invocationContext, params ExecParams) address.Address {
	vmctx.ValidateCaller(pattern.Any{})

	// Dragons: clean this up to match spec
	var state State
	out, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		// create id address
		actorID := state.assignNewID()

		return actorID, nil
	})
	if err != nil {
		panic(err)
	}

	actorID := out.(types.Uint64)

	actorIDAddr, actorAddr := vmctx.CreateActor(actorID, params.ActorCodeCid, params.ConstructorParams)

	_, err = vmctx.State().Transaction(&state, func() (interface{}, error) {
		var err error

		// map id to address and vice versa
		ctx := context.TODO()
		state.AddressMap, err = setID(ctx, vmctx.Runtime().Storage(), state.AddressMap, actorAddr, actorID)
		if err != nil {
			return nil, fmt.Errorf("could not save id by address")
		}

		state.IDMap, err = setAddress(ctx, vmctx.Runtime().Storage(), state.IDMap, actorID, actorAddr)
		if err != nil {
			return nil, fmt.Errorf("could not save address by id")
		}

		return nil, nil
	})
	if err != nil {
		runtime.Abort(exitcode.SysErrInternal)
	}

	return actorIDAddr
}

func lookupIDAddress(vmctx runtime.InvocationContext, state State, addr address.Address) (types.Uint64, error) {
	ctx := context.TODO()
	lookup, err := actor.LoadLookup(ctx, vmctx.Runtime().Storage(), state.AddressMap)
	if err != nil {
		return 0, fmt.Errorf("could not load lookup for cid: %s", state.IDMap)
	}

	var id types.Uint64
	err = lookup.Find(ctx, string(addr.Bytes()), &id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

func setAddress(ctx context.Context, storage runtime.Storage, idMap cid.Cid, actorID types.Uint64, addr address.Address) (cid.Cid, error) {
	lookup, err := actor.LoadLookup(ctx, storage, idMap)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not load lookup for cid: %s", idMap)
	}

	key, err := keyForActorID(actorID)
	if err != nil {
		return cid.Undef, err
	}

	err = lookup.Set(ctx, key, addr)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not set address")
	}

	return lookup.Commit(ctx)
}

func setID(ctx context.Context, storage runtime.Storage, addressMap cid.Cid, addr address.Address, actorID types.Uint64) (cid.Cid, error) {
	lookup, err := actor.LoadLookup(ctx, storage, addressMap)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not load lookup for cid: %s", addressMap)
	}

	err = lookup.Set(ctx, string(addr.Bytes()), actorID)
	if err != nil {
		return cid.Undef, fmt.Errorf("could not set id")
	}

	return lookup.Commit(ctx)
}

func keyForActorID(actorID types.Uint64) (string, error) {
	key, err := encoding.EncodeDeprecated(actorID)
	if err != nil {
		return "", fmt.Errorf("could not encode actor id: %d", actorID)
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
