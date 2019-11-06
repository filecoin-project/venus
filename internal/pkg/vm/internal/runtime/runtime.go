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
	// Review: why is this taking an offset? verify size of offset
	Randomness(epoch Epoch, offset uint64) Randomness
	// Review: the spec still has a TODO on this
	// Review: not sure why the original has a messagereceipt instead of an InvocOutput
	Send(input InvocInput) InvocOutput
	// Review: how does this work? What qualifies as an "error"?
	// Review: how is this any different than the caller just ignoring a returned error?
	// SendAllowingErrors(input msg.InvocInput) msg.MessageReceipt

	// Review: we might want a global Storage() Storage here
}

// InvocationContext is passed to the actors on each method call.
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

	// Review: if this is just for the `init` actor then it should not be here.
	// Review: the init actor is a special system actor, its ok for it to have a different interface with a bit more power.
    // CreateActor(
    //     cid                actor.StateCID
    //     a                  addr.Address
    //     constructorParams  actor.MethodParams
    // )

	// Review: we provide this through the Storage()
    // IpldGet(c ipld.CID) union {Bytes, error}
	// IpldPut(x ipld.Object) ipld.CID
	// Review: is the expectation fo storage to be global or actor specific? this has security implications down the line
	Storage() Storage
}

type innitInvocationContext interface {
	InvocationContext
	// Review: the spec has no error on this? what if the addr is already taken?
	CreateActor(cid cid.Cid, addr address.Address, constructorParams MethodParams)
}

// Storage2 defines the storage module exposed to actors.
// Review: check the old `Storage` for some more comments on why this looks like it does
type Storage2 interface {
	Put(interface{}) (cid.Cid, error)
	Get(cid.Cid, interface{}) error
}

// Epoch is the current chain epoch.
// Dragons: this is just to match names on the VM, Height got renamed to Epoch on the spec
// TODO: rename types.BlockHeight to block.Epoch (issue: ???)
type Epoch types.BlockHeight

// Randomness is a string of random bytes
type Randomness []byte

// InvocInput are the params to invoke a method in an Actor.
// Review: why is this a struct instead of arguments?
type InvocInput struct {
    To      address.Address
    Method  types.MethodID
    Params  MethodParams
    Value   types.AttoFIL
}

// ExitCode is the exit code of a method executing inside the VM.
// Review: do we have reserved codes?
// TODO: this has type UVarint in the spec
type ExitCode types.Int64

// InvocOutput is waht os returned by an actor executing a method.
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
	// Review: this is not in the spec anymore
	Message() *types.UnsignedMessage

	// Review: the spec splits this up and brings the get/put to be first class
	Storage() Storage

	// Review: changed from arguments to a struct
	Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)

	// Review: this is not in the spec anymore
	AddressForNewActor() (address.Address, error)

 	// Review: renamed
	BlockHeight() *types.BlockHeight

	// Review: this is not in the spec anymore
	MyBalance() types.AttoFIL

	// Review: this changed a bit, not sure why it cant be determined off the result of `Caller()`
	IsFromAccountActor() bool

	// Review: this is not in the spec anymore
	Charge(cost types.GasUnits) error

	// Review: this got renamed and gained an offset arg
	SampleChainRandomness(sampleHeight *types.BlockHeight) ([]byte, error)

	// unchanged
	CreateNewActor(addr address.Address, code cid.Cid, initalizationParams interface{}) error

	// Review: this is not in the spec anymore
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
