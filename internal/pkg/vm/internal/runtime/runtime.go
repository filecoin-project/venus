package runtime

import (
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
)

// Runtime has operations in the VM that are exposed to all actors.
type Runtime interface {
	// CurrentEpoch is the current chain epoch.
	CurrentEpoch() abi.ChainEpoch

	NtwkVersion() network.Version
}

// InvocationContext is passed to the actors on each method call.
type InvocationContext interface {
	// Runtime exposes some methods on the runtime to the actor.
	Runtime() Runtime
	// Store is the raw store for IPLD objects.
	//
	// Note: this is required for custom data structures.
	Store() specsruntime.Store
	// Message contains information available to the actor about the executing message.
	Message() specsruntime.Message
	// ValidateCaller validates the caller against a patter.
	//
	// All actor methods MUST call this method before returning.
	ValidateCaller(CallerPattern)
	// StateHandle handles access to the actor state.
	State() specsruntime.StateHandle
	// Send allows actors to invoke methods on other actors
	Send(toAddr address.Address, methodNum abi.MethodNum, params cbor.Marshaler, value abi.TokenAmount, out cbor.Er) exitcode.ExitCode
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
	// Creates a reorg-stable address for a new actor.

	NewActorAddress() address.Address
	// Create an actor in the state tree.
	//
	// This will allocate an ID address for the actor and call its `Constructor()` method.
	//
	// WARNING: May only be called by InitActor.
	CreateActor(codeID cid.Cid, addr address.Address)

	DeleteActor(beneficiary address.Address)
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
