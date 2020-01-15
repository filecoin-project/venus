package message

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gas"
)

// Receipt is what is returned by executing a message on the vm.
type Receipt struct {
	ExitCode    exitcode.ExitCode
	ReturnValue []byte
	GasUsed     gas.Unit
}

// Ok returns an empty succesfull result.
func Ok() Receipt {
	return Receipt{
		ExitCode:    0,
		ReturnValue: nil,
		GasUsed:     gas.Zero,
	}
}

// Value returns a successful code with the value encoded.
//
// Callers do NOT need to encode the value before calling this method.
func Value(obj interface{}) Receipt {
	aux, err := encoding.Encode(obj)
	if err != nil {
		return Receipt{ExitCode: exitcode.EncodingError}
	}

	return Receipt{
		ExitCode:    0,
		ReturnValue: aux,
		GasUsed:     gas.Zero,
	}
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode, gasAmount gas.Unit) Receipt {
	return Receipt{
		ExitCode:    exitCode,
		ReturnValue: nil,
		GasUsed:     gasAmount,
	}
}

// WithGas sets the gas used.
func (r Receipt) WithGas(amount gas.Unit) Receipt {
	r.GasUsed = amount
	return r
}
