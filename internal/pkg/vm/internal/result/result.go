package result

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
)

// InvocOutput is what is returned by an actor executing a method.
type InvocOutput struct {
	ExitCode    exitcode.ExitCode
	ReturnValue []byte
}

// Success returns an empty succesfull result.
func Success() InvocOutput {
	return InvocOutput{ExitCode: 0}
}

// Value returns a successful code with the value encoded.
//
// Callers do NOT need to encode the value before calling this method.
func Value(obj interface{}) InvocOutput {
	aux, err := encoding.Encode(obj)
	if err != nil {
		return InvocOutput{ExitCode: exitcode.EncodingError}
	}

	return InvocOutput{
		ExitCode:    0,
		ReturnValue: aux,
	}
}

// Error returns an error output.
func Error(exitCode exitcode.ExitCode) InvocOutput {
	return InvocOutput{ExitCode: exitCode}
}
