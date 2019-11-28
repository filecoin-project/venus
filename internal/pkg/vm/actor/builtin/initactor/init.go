package initactor

import (
	"context"
	"math/big"
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
)

// Actor is the builtin actor responsible for network initialization.
// More details on future responsibilities can be found at https://github.com/filecoin-project/specs/blob/master/actors.md#init-actor.
type Actor struct{}

// State is the init actor's storage.
type State struct {
	Network    string
	AddressMap cid.Cid `refmt:",omitempty"`
	IdMap      cid.Cid `refmt:",omitempty"`
	NextID     types.Uint64
}

// assignNewID returns the nextID and increments the counter
func (s *State) assignNewID() types.Uint64 {
	id := s.NextID
	s.NextID++
	return id
}

// Actor methods
const (
	Exec                 types.MethodID = 3
	GetActorIDForAddress types.MethodID = 4
	// currently unspecified
	GetAddressForActorID types.MethodID = iota + 32
	GetNetwork
)

// NewActor returns a init actor.
func NewActor() *actor.Actor {
	return actor.NewActor(types.InitActorCodeCid, types.ZeroAttoFIL)
}

//
// ExecutableActor impl for Actor
//

// Ensure InitActor is an ExecutableActor at compile time.
var _ dispatch.ExecutableActor = (*Actor)(nil)

var signatures = dispatch.Exports{
	GetNetwork: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.String},
	},
	Exec: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Cid, abi.Parameters},
		Return: []abi.Type{abi.Address},
	},
	GetActorIDForAddress: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{abi.Integer},
	},
	GetAddressForActorID: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{abi.Integer},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case Exec:
		return reflect.ValueOf((*Impl)(a).Exec), signatures[Exec], true
	case GetActorIDForAddress:
		return reflect.ValueOf((*Impl)(a).GetActorIDForAddress), signatures[GetActorIDForAddress], true
	case GetAddressForActorID:
		return reflect.ValueOf((*Impl)(a).GetAddressForActorID), signatures[GetAddressForActorID], true
	case GetNetwork:
		return reflect.ValueOf((*Impl)(a).GetNetwork), signatures[GetNetwork], true
	default:
		return nil, nil, false
	}
}

// InitializeState for init actor.
func (*Actor) InitializeState(storage runtime.Storage, params interface{}) error {
	network, ok := params.(string)
	if !ok {
		return errors.NewRevertError("init actor network parameter is not a string")
	}

	initStorage := &State{
		Network: network,
		NextID:  100,
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

// minerInvocationContext has some special sauce for the miner.
type invocationContext interface {
	runtime.InvocationContext
	LegacyAddressForNewActor() (address.Address, error)
	LegacyCreateNewActor(addr address.Address, code cid.Cid) error
}

// Impl is the VM implementation of the actor.
type Impl Actor

// GetNetwork returns the network name for this network
func (*Impl) GetNetwork(ctx runtime.InvocationContext) (string, uint8, error) {
	if err := ctx.Charge(actor.DefaultGasCost); err != nil {
		return "", internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	err := actor.ReadState(ctx, &state)
	if err != nil {
		return "", errors.CodeError(err), err
	}

	return state.Network, 0, nil
}

// GetActorIDForAddress looks up the actor id for a filecoin address.
func (a *Impl) GetActorIDForAddress(rt runtime.InvocationContext, addr address.Address) (*big.Int, uint8, error) {
	if err := rt.Charge(actor.DefaultGasCost); err != nil {
		return big.NewInt(0), internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	err := actor.ReadState(rt, &state)
	if err != nil {
		return big.NewInt(0), errors.CodeError(err), err
	}

	ctx := context.TODO()
	lookup, err := actor.LoadLookup(ctx, rt.Runtime().Storage(), state.AddressMap)
	if err != nil {
		return big.NewInt(0), errors.CodeError(err), errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", state.IdMap)
	}

	var id types.Uint64
	err = lookup.Find(ctx, addr.String(), &id)
	if err != nil {
		if err == hamt.ErrNotFound {
			return big.NewInt(0), errors.CodeError(err), errors.RevertErrorWrapf(err, "actor id not found for address: %s", addr.String())
		}
		return big.NewInt(0), errors.CodeError(err), errors.FaultErrorWrap(err, "could not lookup actor id")
	}

	return big.NewInt(int64(id)), 0, nil
}

// GetAddressForActorID looks up the address for an actor id.
func (a *Impl) GetAddressForActorID(rt runtime.InvocationContext, actorID types.Uint64) (address.Address, uint8, error) {
	if err := rt.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	err := actor.ReadState(rt, &state)
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	ctx := context.TODO()
	lookup, err := actor.LoadLookup(ctx, rt.Runtime().Storage(), state.IdMap)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", state.IdMap)
	}

	key, err := a.keyForActorID(actorID)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrapf(err, "could not encode actor id: %d", actorID)
	}

	var addr address.Address
	err = lookup.Find(ctx, key, &addr)
	if err != nil {
		if err == hamt.ErrNotFound {
			return address.Undef, errors.CodeError(err), errors.RevertErrorWrapf(err, "actor address not found for id: %d", actorID)
		}
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrap(err, "could not lookup actor address")
	}

	return addr, 0, nil
}

// Exec creates a new builtin actor.
func (a *Impl) Exec(rt invocationContext, codeCID cid.Cid, params []interface{}) (address.Address, uint8, error) {
	if err := rt.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	if !a.isBuiltinActor(codeCID) {
		return address.Undef, 1, errors.NewRevertError("cannot launch actor instance that is not a builtin actor")
	}

	if a.isSingletonActor(codeCID) {
		return address.Undef, 1, errors.NewRevertError("cannot launch another actor of this type")
	}

	var state State
	err := actor.ReadState(rt, &state)
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	// create more stable address
	actorAddr, err := rt.LegacyAddressForNewActor()
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrapf(err, "could not create address for actor")
	}

	// create id address
	actorID := state.assignNewID()
	idAddr, err := address.NewIDAddress(uint64(actorID))
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrapf(err, "could not create id address with id %d", actorID)
	}

	// map id to address and vice versa
	ctx := context.TODO()
	state.AddressMap, err = a.setId(ctx, rt.Runtime().Storage(), state.AddressMap, actorAddr, actorID)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrap(err, "could not save id by address")
	}

	state.IdMap, err = a.setAddress(ctx, rt.Runtime().Storage(), state.IdMap, actorID, actorAddr)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrap(err, "could not save addres by id")
	}

	// write the state
	err = actor.WriteState(rt, state)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrap(err, "could not write actor state")
	}

	// create new actor keyed by id address
	err = rt.LegacyCreateNewActor(idAddr, codeCID)
	if err != nil {
		return address.Undef, errors.CodeError(err), errors.FaultErrorWrapf(err, "could not create actor with address %s", idAddr.String())
	}

	// send message containing actor's initial balance to construct it with the given params
	_, _, err = rt.Runtime().Send(idAddr, types.ConstructorMethodID, rt.Message().ValueReceived(), params)
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	return actorAddr, 0, nil
}

func (a *Impl) setAddress(ctx context.Context, storage runtime.Storage, idMap cid.Cid, actorID types.Uint64, addr address.Address) (cid.Cid, error) {
	lookup, err := actor.LoadLookup(ctx, storage, idMap)
	if err != nil {
		return cid.Undef, errors.RevertErrorWrapf(err, "could not load lookup for cid: %s", idMap)
	}

	key, err := a.keyForActorID(actorID)
	if err != nil {
		return cid.Undef, err
	}

	err = lookup.Set(ctx, key, addr)
	if err != nil {
		return cid.Undef, errors.FaultErrorWrapf(err, "could not set address")
	}

	return lookup.Commit(ctx)
}

func (a *Impl) setId(ctx context.Context, storage runtime.Storage, addressMap cid.Cid, addr address.Address, actorID types.Uint64) (cid.Cid, error) {
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

func (a *Impl) keyForActorID(actorID types.Uint64) (string, error) {
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
		code.Equals(types.BootstrapMinerActorCodeCid) ||
		code.Equals(types.PaymentBrokerActorCodeCid)
}

func (a *Impl) isSingletonActor(code cid.Cid) bool {
	return code.Equals(types.StorageMarketActorCodeCid) ||
		code.Equals(types.InitActorCodeCid) ||
		code.Equals(types.PaymentBrokerActorCodeCid)
}
