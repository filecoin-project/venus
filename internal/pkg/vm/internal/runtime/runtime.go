package runtime

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)



// Runtime2 is the VM.
// Review: the object that implements this outlives actor calls.
type Runtime2 struct {
	CurerntEpoch() Epoch
	// Review: should we turn the offset into a RandomnessEvent?
	Randomness(epoch Epoch, offset uint64) Randomness
	// Review: not sure why the original has a messagereceipt instead of an InvocOutput
	Send(input InvocInput) InvocOutput
	// Review: how does this work? What qualifies as an "error"?
	// Review: how is this any different than the caller just ignoring a returned error?
	// SendAllowingErrors(input msg.InvocInput) msg.MessageReceipt

	// Review: we might want a global Storage() Storage here
}

// InvocationContext is passed to the actors on each method call.
// Review: this type + Runtime2 is mostly what the spec defines as Runtime.
type InvocationContext interface {
	// Review: grouped the VM level things out of the invocation itself
	Runtime() Runtime2
	Caller() address.Address
	// Review: (is in spec, dont think we need it) idem to Caller().Equal(addr) :/
	// ValidateCallerIs(caller addr.Address)
	// Review: not sure what all this pattern matching adds up to, the "singleton" pattern does an equality check
	// ValidateCallerMatches(CallerPattern)
	// Review: the spec handles state and state changes a bit differently
	// Review: our Storage is a bit of a mix of AcquireState and the Ipld{Get-Put}
	// AcquireState() ActorStateHandle

	// Review: The spec suggest handling return construction by the runtime, interesting but not strictly necessary
	// SuccessReturn()     msg.InvocOutput
	// ValueReturn(Bytes)  msg.InvocOutput
	// ErrorReturn(exitCode exitcode.ExitCode) msg.InvocOutput

	// Review: how does this work? is the spec expecting us to Panic? or just terminate the message execution?
	// Review: this should be `Abort(string) -> msg.InvocOutput` if we follow the pattern
	// Abort(string)

	// Review: this is a bit trickier than abort in Go. This is a conditional early exit on the function which we cannot do.
	// Review: `if <cond> { return AssertFailure("blah") }` is a bit closer to go if we are to try to follow this pattern.
	// Assert(bool)

	// Review: we didnt have this before, why does the actor need to know the value supplied?
	ValueSupplied() types.AttoFIL

	// Review: is the expectation fo storage to be global or actor specific? this has security implications down the line
	Storage() Storage

	// Dragons: this was not in the spce, but it was likely a bug
	// Note: @jzimmerman I think that's an error in the spec -- balance should be accessible via the runtime.
	Balance() types.AttoFIL
}

// initInvocationContext is a more powerfull context for the "init" actor.
type initInvocationContext interface {
	InvocationContext
	// Review: added an error that is not yet on the spec but should be
	// Note: @jzimmerman: "Agreed, this should support an error return."
	CreateActor(cid cid.Cid, addr address.Address, constructorParams MethodParams) error
}

// Storage2 defines the storage module exposed to actors.
// Review: simpler API (check type Storage for comments on the things that got removed)
type Storage2 interface {
	Put(interface{}) (cid.Cid, error)
	Get(cid.Cid, interface{}) error
}

// Epoch is the current chain epoch.
// Dragons: this is just to match names on the VM, Height got renamed to Epoch on the spec
// TODO: rename types.BlockHeight to block.Epoch (issue: ???)
type Epoch types.BlockHeight

// Randomness is a string of random bytes
// Note: placeholder for matching newtypes with the spec.
type Randomness []byte

// InvocInput are the params to invoke a method in an Actor.
// Review: unless anyone in @go-filecoin has a strong preference for arguments, we are chaging to the struct the spec suggests.
// Note: @jzimmerman mentioned we might want to reffer to the inputs by Cid in the future
type InvocInput struct {
    To      address.Address
    Method  types.MethodID
    Params  MethodParams
    Value   types.AttoFIL
}

// ExitCode is the exit code of a method executing inside the VM.
type ExitCode types.Int64

// InvocOutput is waht os returned by an actor executing a method.
// Review: multiple returns vs a return struct. There are some minor implications with swapping vm implementations written in another language.
type InvocOutput struct {
	ExitCode     ExitCode
    ReturnValue  []byte
}

// MethodParam is the parameter to an Actor method already encoded for execution in the VM.
type MethodParam []byte
// MethodParams is a list of `MethodParam` to be passed into an Actor method.
type MethodParams []MethodParam

// Runtime defines the ABI interface exposed to actors.
type Runtime interface {
	// Review: this is not in the spec ~anymore~ yet..
	// Note: @jzimmerman: still a WIP, things like the miner actor might need it
	Message() *types.UnsignedMessage

	// The spec splits this up and brings the get/put to be first class,
	// Review: go-filecoin should keep it as a composition
	Storage() Storage

	// Changed from arguments to a struct
	// Note: we will likely go with the struct, check comments on `type InvocInput`
	Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)

	// This is not in the spec anymore
	// Note: @jzimmerman: this functionality is delicate enough security-wise (especially with respect to different addresses on different forks) that I think it makes sense not to expose it directly.
	AddressForNewActor() (address.Address, error)

 	// Review: renamed and has a new type
	BlockHeight() *types.BlockHeight

	// This is not in the spec anymore, but it should be
	// Note: @jzimmerman I think that's an error in the spec -- balance should be accessible via the runtime.
	MyBalance() types.AttoFIL

	// got replaceed by ValidateCallerIs and ValidateCallerMAtches
	IsFromAccountActor() bool

	// This is not in the spec anymore.
	// Note: @jzimmerman: "Gas accounting is still WIP"
	// TODO: come back to this when Gas accounting lands on spec
	Charge(cost types.GasUnits) error

	// Note: this got renamed and gained an offset arg
	SampleChainRandomness(sampleHeight *types.BlockHeight) ([]byte, error)

	// unchanged
	CreateNewActor(addr address.Address, code cid.Cid, initalizationParams interface{}) error

	// Review: this is not in the spec anymore
	// Note: the spec seems to do a direct call, that has some implications (see github comments).
	Verifier() verification.Verifier
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
