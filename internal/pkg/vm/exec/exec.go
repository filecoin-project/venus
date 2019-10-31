package exec

import (
	"context"
	"reflect"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
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
type Exports map[types.MethodID]*FunctionSignature

// Has checks if the given method is an exported method.
func (e Exports) Has(method types.MethodID) bool {
	_, ok := e[method]
	return ok
}

// Method is a callable pointer to an executable method in an actor implementation.
type Method interface {
	Call(in []reflect.Value) []reflect.Value
	Type() reflect.Type
}

// TODO fritz require actors to define their exit codes and associate
// an error string with them.

// ExecutableActor is the interface all builtin actors have to implement.
type ExecutableActor interface {
	Method(id types.MethodID) (Method, *FunctionSignature, bool)
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
	Message() *types.UnsignedMessage
	Storage() Storage
	Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error)
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

// KV is a Key/Value pair correctly unmarshaled from a hamt based on the lookups
// type
type KV struct {
	Key      string
	RawValue []byte
}

// ValueCallbackFunc is called when iterating nodes with a HAMT lookup in order
// with a key and a decoded value
type ValueCallbackFunc func(k string, v interface{}) error

// Lookup defines an internal interface for actor storage.
type Lookup interface {
	Find(ctx context.Context, k string, out interface{}) error
	Set(ctx context.Context, k string, v interface{}) error
	Commit(ctx context.Context) (cid.Cid, error)
	Delete(ctx context.Context, k string) error
	IsEmpty() bool
	ForEachValue(ctx context.Context, valueType interface{}, callback ValueCallbackFunc) error
}
