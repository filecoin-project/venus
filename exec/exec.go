package exec

import (
	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// ErrorCode is an enumerated set of errors that VMContext methods can return.
type ErrorCode uint8

const (
	// Ok indicates that no error occurred
	Ok = ErrorCode(0)

	// ErrDecode indicates that a chunk an actor tried to write could not be decoded
	ErrDecode = ErrorCode(33)

	// ErrDanglingPointer indicates that an actor attempted to commit a pointer to a non-existent chunk
	ErrDanglingPointer = ErrorCode(34)

	// ErrStaleHead indicates that an actor attempted to commit over a stale chunk
	ErrStaleHead = ErrorCode(35)
)

// Exports describe the public methods of an actor.
type Exports map[string]*FunctionSignature

// Has checks if the given method is an exported method.
func (e Exports) Has(method string) bool {
	_, ok := e[method]
	return ok
}

// TODO fritz require actors to define their exit codes and associate
// an error string with them.

// ExecutableActor is the interface all builtin actors have to implement.
type ExecutableActor interface {
	Exports() Exports
	NewStorage() interface{}
}

// ExportedFunc is the signature an exported method of an actor is expected to have.
type ExportedFunc func(ctx VMContext) ([]byte, uint8, error)

// FunctionSignature describes the signature of a single function.
// TODO: convert signatures into non go types, but rather low level agreed up types
type FunctionSignature struct {
	// Params is a list of the types of the parameters the function expects.
	Params []abi.Type
	// Return is the type of the return value of the function.
	Return []abi.Type
}

// VMContext defines the ABI interface exposed to actors.
type VMContext interface {
	Message() *types.Message
	Storage() Storage
	Send(to types.Address, method string, value *types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	AddressForNewActor() (types.Address, error)
	BlockHeight() *types.BlockHeight
	IsFromAccountActor() bool

	// TODO: replace with proper init actor
	TEMPCreateActor(addr types.Address, act *types.Actor) error

	// TODO: Remove these when Storage above is completely implemented
	ReadStorage() ([]byte, error)
	WriteStorage(memory []byte) error
}

// Storage defines the storage module exposed to actors.
type Storage interface {
	// TODO: Forgot that Put() can fail in the spec, need to update.
	Put([]byte) (*cid.Cid, ErrorCode)
	Get(*cid.Cid) ([]byte, bool, error)
	Commit(*cid.Cid, *cid.Cid) ErrorCode
	Head() *cid.Cid
}
