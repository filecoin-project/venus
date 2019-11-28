package runtime

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
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
	// Dragons: cleanup to match new vm expectations
	Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	// Storage is the raw store for IPLD objects.
	//
	// Note: this is required for custom data structures.
	Storage() Storage
}

// MessageInfo contains information available to the actor about the executing message.
type MessageInfo interface {
	// BlockMiner is the address for the actor who mined the block in which the initial on-chain message appears.
	BlockMiner() address.Address
	// ValueReceived is the amount of FIL received by this actor during this method call.
	//
	// Note: the value has already been deposited on the actors account and is reflected in the balance.
	ValueReceived() types.AttoFIL
	// Caller is the immediate caller to the current executing method.
	Caller() address.Address
}

// InvocationContext is passed to the actors on each method call.
type InvocationContext interface {
	// Runtime exposes some methods on the runtime to the actor.
	Runtime() Runtime
	// Message contains information available to the actor about the executing message.
	Message() MessageInfo
	// ValidateCaller validates the caller against a patter.
	//
	// All actor methods MUST call this method before returning.
	ValidateCaller(CallerPattern)
	// StateHandle handles access to the actor state.
	StateHandle() ActorStateHandle
	// Balance is the current balance on the current actors account.
	//
	// Note: the value received for this invocation is already reflected on the balance.
	Balance() types.AttoFIL
	// Charge allows actor code to charge extra.
	//
	// This method should be rarely used, the VM takes care of charging the gas on calls.
	//
	// Methods with extra complexity that is not accounted by other means (i.e. external calls, storage calls)
	// will have to charge extra.
	Charge(units types.GasUnits) error
}

// ExtendedInvocationContext is a set of convenience functions built on top external ABI calls.
//
// Actor code should not be using this interface directly.
//
// Note: This interface is intended to document the full set of available operations
// and ensure the context implementation exposes them.
type ExtendedInvocationContext interface {
	InvocationContext
	// Dragons: add new CreateActor (this is just for the init actor)
	// CreateActor(code cid.Cid, params []interface{}) address.Address
	// Dragons: add new VerifySignature (mileage on the arg types may vary)
	// VerifySignature(signer address.Address, signature filcrypto.Signature, msg filcrypto.Message) bool
}

// LegacyInvocationContext are the methods from the old VM we have not removed yet.
//
// WARNING: Every method in this interface is to be considered DEPRECATED.
// Dragons: this methods are legacy and have not been ported to the new VM semantics.
type LegacyInvocationContext interface {
	InvocationContext
	LegacyMessage() *types.UnsignedMessage
	LegacyCreateNewActor(addr address.Address, code cid.Cid) error
	LegacyAddressForNewActor() (address.Address, error)
	LegacyVerifier() verification.Verifier
}

// ActorStateHandle handles the actor state, allowing actors to lock on the state.
type ActorStateHandle interface {
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

// Randomness is a string of random bytes
type Randomness []byte

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
	// Dragons: move out after cleaning up the actor state construction
	Head() cid.Cid
	Put(interface{}) (cid.Cid, error)
	// Dragons: move out after cleaning up the actor state construction
	CidOf(interface{}) (cid.Cid, error)
	// Dragons: this interface is wrong, the caller does not know how the object got serialized (Put/1 takes an interface{} not bytes)
	// TODO: change to `Get(cid, interface{}) error`
	Get(cid.Cid) ([]byte, error)
	// Dragons: move out after cleaning up the actor state construction
	Commit(cid.Cid, cid.Cid) error
}
