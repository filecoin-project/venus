// Package external is a temporary package that will only live while we are cleaning up vm into vm
//
// All the calls into this should either not depend on this, or we should find a proper package for each.
package external

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
)

// FunctionSignature describes the signature of a single function.
// TODO: convert signatures into non go types, but rather low level agreed up types
type FunctionSignature struct {
	// Params is a list of the types of the parameters the function expects.
	Params []abi.Type
	// Return is the type of the return value of the function.
	Return []abi.Type
}
