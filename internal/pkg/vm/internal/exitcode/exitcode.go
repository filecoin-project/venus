package exitcode

import "github.com/filecoin-project/go-filecoin/internal/pkg/types"

// ExitCode is the exit code of a method executing inside the VM.
type ExitCode types.Uint64

// Exit codes.
const (
	// OK is the success return value, similar to unix exit code 0.
	Ok ExitCode = ExitCode(0)

	// ActorNotFound represents a failure to find an actor.
	ActorNotFound = ExitCode(1)

	// ActorCodeNotFound represents a failure to find the code for a
	// particular actor in the VM registry.
	ActorCodeNotFound = ExitCode(2)

	// InvalidMethod represents a failure to find a method in an actor
	InvalidMethod = ExitCode(3)

	// InsufficientFunds represents a failure to apply a message, as
	// it did not carry sufficient funds for its application.
	InsufficientFunds = ExitCode(4)

	// InvalidCallSeqNum represents a message invocation out of sequence.
	// This happens when message.CallSeqNum is not exactly actor.CallSeqNum + 1
	InvalidCallSeqNum = ExitCode(5)

	// OutOfGasError is returned when the execution of an actor method
	// (including its subcalls) uses more gas than initially allocated.
	OutOfGas = ExitCode(6)

	// RuntimeAPIError is returned when an actor method invocation makes a call
	// to the runtime that does not satisfy its preconditions.
	RuntimeAPIError = ExitCode(7)

	// MethodPanic is returned when an actor method invocation calls rt.Abort.
	MethodAbort = ExitCode(8)

	// MethodPanic is returned when the runtime intercepts a panic within
	// an actor method invocation (not via rt.Abort).
	MethodPanic = ExitCode(9)

	// MethodSubcallError is returned when an actor method's Send call has
	// returned with a failure error code (and the Send call did not specify
	// to ignore errors).
	MethodSubcallError = ExitCode(10)

	// Runtime failed to encode the object.
	EncodingError = ExitCode(11)
)

// IsSuccess returns `True` if the exit code represents success.
func (code ExitCode) IsSuccess() bool {
	return code == Ok
}

// IsError returns `True` if the exit code is an error.
func (code ExitCode) IsError() bool {
	return code != Ok
}
