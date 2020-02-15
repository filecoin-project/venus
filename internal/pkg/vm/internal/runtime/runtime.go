package runtime

import (
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
)

// Runtime has operations in the VM that are exposed to all actors.
type Runtime interface {
	// CurrentEpoch is the current chain epoch.
	CurrentEpoch() abi.ChainEpoch
	// Randomness gives the actors access to sampling peudo-randomess from the chain.
	Randomness(epoch abi.ChainEpoch) abi.Randomness
	// Storage is the raw store for IPLD objects.
	//
	// Note: this is required for custom data structures.
	Storage() Storage
}

// InvocationContext is passed to the actors on each method call.
type InvocationContext interface {
	// Runtime exposes some methods on the runtime to the actor.
	Runtime() Runtime
	// Message contains information available to the actor about the executing message.
	Message() specsruntime.Message
	// ValidateCaller validates the caller against a patter.
	//
	// All actor methods MUST call this method before returning.
	ValidateCaller(CallerPattern)
	// StateHandle handles access to the actor state.
	State() ActorStateHandle
	// Send allows actors to invoke methods on other actors
	Send(to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) interface{}
	// Balance is the current balance on the current actors account.
	//
	// Note: the value received for this invocation is already reflected on the balance.
	Balance() abi.TokenAmount
}

// ExtendedInvocationContext is a set of convenience functions built on top external ABI calls.
//
// Actor code should not be using this interface directly.
//
// Note: This interface is intended to document the full set of available operations
// and ensure the context implementation exposes them.
type ExtendedInvocationContext interface {
	InvocationContext
	// Create an actor in the state tree.
	//
	// This will determine a reorg "stable" address for the actor and call its `Constructor()` method.
	//
	// WARNING: May only be called by InitActor.
	CreateActor(actorID types.Uint64, code cid.Cid, constructorParams []byte) (address.Address, address.Address)
	// VerifySignature cryptographically verifies the signature.
	//
	// This methods returns `True` when 'signature' is signed hash of 'msg'
	// using the public key belonging to the `signer`.
	VerifySignature(signer address.Address, signature types.Signature, msg []byte) bool
}

// ActorStateHandle handles the actor state, allowing actors to lock on the state.
// Dragons: delete once the new actors are in
type ActorStateHandle interface {
	// Create initializes the state to the given value.
	//
	// This operation is only valid if the value has never been set before.
	Create(obj interface{})
	// Readonly loads a readonly copy of the state into the argument.
	//
	// Any modification to the state is illegal and will result in an `Abort`.
	Readonly(obj interface{})
	// Transaction loads a mutable version of the state into the `obj` argument and protects
	// the execution from side effects.
	//
	// The second argument is a function which allows the caller to mutate the state.
	//
	// The new state will be committed if there are no errors returned.
	// Note: if an error is returned, the state changes will be DISCARDED and the reference will mutate to
	// 		 be equivalent to value of doing a Readonly().
	//
	// WARNING: If the state is modified AFTER the function returns, the execution will Abort.
	//	        The state is mutable ONLY inside the lambda.
	//
	// Transaction can be thought of as having the following signature:
	//
	// `Transaction(F) -> (T, Error) where F: Fn(S) -> (T, error), S: ActorState`.
	//
	// Note: the actual Go signature is a bit different due to the lack of type system magic.
	//		 In order to know `S`, the actual signature looks like:
	//       `Transaction(S, F) where S: ActorState, F: Fn() -> (T, Error)`.
	//
	// # Usage
	//
	// ```go
	// var state SomeState
	// ret, err := ctx.StateHandke().Transaction(&state, func() (interface{}, error) {
	//   // make some changes
	//	 st.ImLoaded = True
	//   return st.Thing, nil
	// })
	// // state.ImLoaded = False // BAD!! state is readonly outside the lambda, it will panic
	// ```
	Transaction(obj interface{}, f func() (interface{}, error)) (interface{}, error)
}

// PatternContext is the context a pattern gets access to in order to determine if the caller matches.
type PatternContext interface {
	CallerCode() cid.Cid
	CallerAddr() address.Address
}

// CallerPattern checks if the caller matches the pattern.
type CallerPattern interface {
	// IsMatch returns "True" if the patterns matches
	IsMatch(ctx PatternContext) bool
}

// ExecutionPanic is used to abort vm execution with an exit code.
type ExecutionPanic struct {
	msg  string
	code exitcode.ExitCode
}

// Code is the code used to abort the execution (see: `Abort()`).
func (p ExecutionPanic) Code() exitcode.ExitCode {
	return p.code
}

func (p ExecutionPanic) String() string {
	if p.msg != "" {
		return p.msg
	}
	return fmt.Sprintf("ExitCode(%d)", p.Code())
}

// Abort aborts the VM execution and sets the executing message return to the given `code`.
func Abort(code exitcode.ExitCode) {
	panic(ExecutionPanic{code: code})
}

// Abortf will stop the VM execution and return an the error to the caller.
func Abortf(code exitcode.ExitCode, msg string, args ...interface{}) {
	panic(ExecutionPanic{code: code, msg: fmt.Sprintf(msg, args...)})
}

// Storage defines the storage module exposed to actors.
type Storage interface {
	// Put stores an object and returns its content-addressable ID.
	Put(interface{}) cid.Cid
	// Get retrieves an object, returns true if it exists.
	Get(cid cid.Cid, obj interface{}) bool
	// GetRaw retrieves the raw bytes stored, returns true if it exists.
	GetRaw(cid cid.Cid) ([]byte, bool)
	// CidOf returns the content-addressable ID of an object WITHOUT storing it.
	CidOf(interface{}) cid.Cid
}
