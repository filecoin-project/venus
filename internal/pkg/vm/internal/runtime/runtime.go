package runtime

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Runtime has operations in the VM that are exposed to all actors.
type Runtime interface {
	// CurrentEpoch is the current chain epoch.
	CurrentEpoch() types.BlockHeight
	// Randomness gives the actors access to sampling peudo-randomess from the chain.
	Randomness(epoch types.BlockHeight, offset uint64) Randomness
	// Send allows actors to invoke methods on other actors
	// Send(input InvocInput) result.InvocOutput
	// Dragons: match signature for this PR, change to struct on next PR
	Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	// Review: should be add `Abort` and `Assert` here?
}

// InvocationContext is passed to the actors on each method call.
type InvocationContext interface {
	// Runtime exposes some methods on the runtime to the actor.
	Runtime() Runtime
	// ValidateCaller validates the caller against a patter.
	//
	// All actor methods MUST call this method before returning.
	ValidateCaller(CallerPattern)
	// Caller is the immediate caller to the current executing method.
	Caller() address.Address
	// StateHandle handles access to the actor state.
	StateHandle() ActorStateHandle
	// ValueReceived is the amount of FIL received by this actor during this method call.
	//
	// Note: the value is already been deposited on the actors account and is reflected on the balance.
	ValueReceived() types.AttoFIL
	// Balance is the current balance on the current actors account.
	//
	// Note: the value received for this invocation is already reflected on the balance.
	Balance() types.AttoFIL
	// Review: we seem to require this
	Storage() Storage
	// Dragons: here just to avoid deleting a lot of lines while we wait for the new Gas Accounting to land
	Charge(cost types.GasUnits) error
}

// ActorStateHandle handles the actor state, allowing actors to lock on the state.
type ActorStateHandle interface {
	// Take loads the state and locks it.
	//
	// Any futre calls to `Take` on this actors state will `abort` the execution.
	// Review: for @spec, the impl of `Take` is panic'ing on the second call on the SAME object instance (this objects are not kept around, so it is always a new instance..).
	Take(interface{})
	// UpdateRelease updates the actor state and releases the lock on it.
	//
	// No future calls to `Take` are allowed on this object.
	// Review: for @spec, this method can currently be called without having called `Take` first.
	UpdateRelease(interface{})
	// Release asserts that the state has not changed and releases the lock on it.
	//
	// No future calls to `Take` are allowed on this object.
	// Review: for @spec, this method can currently be called without having called `Take` first.
	// Review: for @spec, why is this method required?
	Release(interface{})
}

// Randomness is a string of random bytes
type Randomness []byte

// InvocInput are the params to invoke a method in an Actor.
type InvocInput struct {
	To     address.Address
	Method types.MethodID
	Params MethodParams
	Value  types.AttoFIL
}

// MethodParam is the parameter to an actor method.
type MethodParam []byte

// MethodParams is a list of `MethodParam` to be passed into an Actor method.
type MethodParams []MethodParam

// PatternContext is the context a pattern gets access to in order to determine if the caller matches.
type PatternContext interface {
	Code() cid.Cid
}

// CallerPattern checks if the caller matches the pattern.
type CallerPattern interface {
	// IsMatch returns "True" if the patterns matches
	IsMatch(ctx PatternContext) bool
}

// AbortPanicError is used on panic when `Abort` is called by actors.
type AbortPanicError struct {
	msg string
}

func (x AbortPanicError) Error() string {
	return x.msg
}

// Abort will stop the VM execution and return an abort error to the caller.
func Abort(msg string) {
	panic(AbortPanicError{msg: msg})
}

// Assert will abort if the condition is `False` and return an abort error to the caller.
func Assert(cond bool) {
	if !cond {
		Abort("assertion failure in actor code.")
	}
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
	// Review: what situation requires state to change, successfully execute the method, but not to be commited?
	Commit(cid.Cid, cid.Cid) error
}
