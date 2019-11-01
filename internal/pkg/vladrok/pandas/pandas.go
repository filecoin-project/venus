// Package pandas is not for you
// Dragons: all this people need a new home, or the callers need to go do something else
package pandas

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
