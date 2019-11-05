// Package internal has all the things vm and only vm need.
//
// This contents can be slowly placed on the vm internal.
package internal

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
)

// TODO fritz require actors to define their exit codes and associate
// an error string with them.

const (
	// ErrDecode indicates that a chunk an actor tried to write could not be decoded
	ErrDecode = 33
	// ErrDanglingPointer indicates that an actor attempted to commit a pointer to a non-existent chunk
	ErrDanglingPointer = 34
	// ErrStaleHead indicates that an actor attempted to commit over a stale chunk
	ErrStaleHead = 35
	// ErrInsufficientGas indicates that an actor did not have sufficient gas to run a message
	// Dragons: this guy is not following the same pattern, it is missing from the Errors map below
	ErrInsufficientGas = 36
)

// Errors map error codes to revert errors this actor may return
var Errors = map[uint8]error{
	ErrDecode:          errors.NewCodedRevertError(ErrDecode, "State could not be decoded"),
	ErrDanglingPointer: errors.NewCodedRevertError(ErrDanglingPointer, "State contains pointer to non-existent chunk"),
	ErrStaleHead:       errors.NewCodedRevertError(ErrStaleHead, "Expected head is stale"),
}
