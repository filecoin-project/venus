package exec

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// Error represents a storage related error
type Error string

func (e Error) Error() string { return string(e) }

const (
	// ErrDecode indicates that a chunk an actor tried to write could not be decoded
	ErrDecode = 33
	// ErrDanglingPointer indicates that an actor attempted to commit a pointer to a non-existent chunk
	ErrDanglingPointer = 34
	// ErrStaleHead indicates that an actor attempted to commit over a stale chunk
	ErrStaleHead = 35
	// ErrInsufficientGas indicates that an actor did not have sufficient gas to run a message
	ErrInsufficientGas = 36
)

// Errors map error codes to revert errors this actor may return
var Errors = map[uint8]error{
	ErrDecode:          errors.NewCodedRevertError(ErrDecode, "State could not be decoded"),
	ErrDanglingPointer: errors.NewCodedRevertError(ErrDanglingPointer, "State contains pointer to non-existent chunk"),
	ErrStaleHead:       errors.NewCodedRevertError(ErrStaleHead, "Expected head is stale"),
}

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
	InitializeState(storage Storage, initializerData interface{}) error
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
	Send(to address.Address, method string, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
	AddressForNewActor() (address.Address, error)
	BlockHeight() *types.BlockHeight
	MyBalance() types.AttoFIL
	IsFromAccountActor() bool
	Charge(cost types.GasUnits) error
	SampleChainRandomness(sampleHeight *types.BlockHeight) ([]byte, error)

	CreateNewActor(addr address.Address, code cid.Cid, initalizationParams interface{}) error

	Verifier() verification.Verifier
}

// Storage defines the storage module exposed to actors.
type Storage interface {
	Put(interface{}) (cid.Cid, error)
	Get(cid.Cid) ([]byte, error)
	Commit(cid.Cid, cid.Cid) error
	Head() cid.Cid
}

// Lookup defines an internal interface for actor storage.
type Lookup interface {
	Find(ctx context.Context, k string) (interface{}, error)
	Set(ctx context.Context, k string, v interface{}) error
	Commit(ctx context.Context) (cid.Cid, error)
	Delete(ctx context.Context, k string) error
	IsEmpty() bool
	Values(ctx context.Context) ([]*hamt.KV, error)
}
