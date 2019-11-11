package runtime

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Runtime2 is the VM.
type Runtime2 struct {
	CurerntEpoch() Epoch
	// Review: for @go-filecoin, should we turn the offset into a RandomnessEvent?
	Randomness(epoch Epoch, offset uint64) Randomness
	Send(input InvocInput) result.InvocOutput
	// SharedStorage is an IPLD store shared by all actors.
	// Review: for @go-filecoin, we can skip this until we have actor code that actually needs this
	// Review: for @spec, Add expiration (Epoch's) on PUT? (the question is mostly for global, but the local store might want to advise on this too).
	// Review: for @spec, how will gas accounting work on global? PUT fee, GET fee, OWNER Fee?
	SharedStorage() IpldStorage2
}

// DispatchContext is the context the actor dispatcher receives.
// Review: for @go-filecoin, do we want to have the `ctx.ValidateCaller(blahblah)` at the top of each method, or hidden away on the dispatch table?
type DispatchContext interface {
	// Review: for @go-filecoin, simplified the API to be just one method.
	ValidateCaller(CallerPattern)
	// InvocationContext is the context to be passed down the actor method.
	InvocationContext() InvocationContext
}

// InvocationContext is passed to the actors on each method call..
type InvocationContext interface {
	// Review: for @go-filecoin, composition or inheritance?
	Runtime() Runtime2
	Caller() address.Address
	StateHandler() ActorStateHandler

	// Review: for @go-filecoin+@spec, renamed `ValueSupplied` to `ValueReceived`. The "supplied" is not clear by whom.
	ValueReceived() types.AttoFIL
	Balance() types.AttoFIL

	LocalStorage() IpldStorage2
}

// initInvocationContext is a more powerfull context for the "init" actor.
type initInvocationContext interface {
	InvocationContext
	// Review: for @spec, added an error that is not yet on the spec but should be
	// Note: @jzimmerman: "Agreed, this should support an error return."
	CreateActor(cid cid.Cid, addr address.Address, constructorParams MethodParams) error
}

// minerInvocationContext has some special sauce for the miner.
// Review: for @go-filecoin, do we want custom interfaces for the miners instead of grouping a bunch fo methods on the InvocationContext?
type minerInvocationContext interface {
	Verifier() verification.Verifier
}

// ActorStateHandler handles the actor state, allowing actors to lock on the state.
type ActorStateHandler interface {
	// Take returns the `Cid` for the state and locks the state.
	//
	// Any futre calls to `Take` by other instances will block until `Release` or `UpdateRelease` is called.
	// Review: for @spec, the spec impl of `Take` is not doing any blocking, just panic'ing on the second call on the SAME object instance (this objects are not kept around, so it is always a new instance..).
	Take() cid.Cid
	// UpdateRelease updates the actor state and releases the lock on it.
	//
	// No future calls to `Take` are allowed on this object.
	// Review: for @spec, this method can currently be called without having called `Take` first.
	UpdateRelease(newStateCID ActorSubstateCID)
	// Release asserts that the state has not changed and releases the lock on it.
	//
	// No future calls to `Take` are allowed on this object.
	// Review: for @spec, this method can currently be called without having called `Take` first.
	// Review: for @spec, why is this method required?
	Release(checkStateCID ActorSubstateCID)
}

// IpldStorage2 defines the storage module exposed to actors.
type IpldStorage2 interface {
	Put(interface{}) (cid.Cid, error)
	Get(cid.Cid, interface{}) error
}

// CallerPattern checks if the caller matches the pattern.
type CallerPattern interface {
	IsMatch() bool
}

// Epoch is the current chain epoch.
// Dragons: this is just to match names on the VM, Height got renamed to Epoch on the spec
// TODO: rename types.BlockHeight to block.Epoch (issue: ???)
type Epoch types.BlockHeight

// Randomness is a string of random bytes
// Note: placeholder for matching newtypes with the spec.
type Randomness []byte

// InvocInput are the params to invoke a method in an Actor.
type InvocInput struct {
    To      address.Address
    Method  types.MethodID
    Params  MethodParams
    Value   types.AttoFIL
}

// MethodParam is the parameter to an Actor method already encoded for execution in the VM.
type MethodParam []byte
// MethodParams is a list of `MethodParam` to be passed into an Actor method.
type MethodParams []MethodParam

// Abort will stop the VM execution and return an abort error to the caller.
// Review: @go-filecoin, do we want to handle this with panics?
func Abort(msg string) {
	panic("byte me")
}

// Assert will abort if the condition is `False` and return an abort error to the caller.
func Assert(cond bool) {
	if !cond {
		Abort("assertion failure in actor code.")
	}
}

//
// Dragons: this will get deleted, still here while we finish some discussions on the PR.
//

// Runtime defines the ABI interface exposed to actors.
type Runtime interface {
	// Review: this is not in the spec ~anymore~ yet..
	// Note: @jzimmerman: still a WIP, things like the miner actor might need it
	Message() *types.UnsignedMessage
}

// Storage defines the storage module exposed to actors.
type Storage interface {
	Head() cid.Cid
	Put(interface{}) (cid.Cid, error)
	// Dragons: this interface is wrong, the caller does not know how the object got serialized (Put/1 takes an interface{} not bytes)
	// TODO: change to `Get(cid, interface{}) error`
	Get(cid.Cid) ([]byte, error)
	// Review: why is this needed on the actor API? Actor commit is implicit upon a succesfull termination.
	// Review: an explicit Commit() on actor code leads to bugs
	// Review: what situation requires state to change, succesfully execute the method, but not to be commited?
	Commit(cid.Cid, cid.Cid) error
}
