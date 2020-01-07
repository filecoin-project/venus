package message

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
)

// Receipt is what is returned by executing a message on the vm.
type Receipt struct {
	ExitCode    exitcode.ExitCode
	ReturnValue []byte
	GasUsed     types.GasUnits
}

// Ok returns an empty succesfull result.
func Ok() Receipt {
	return Receipt{
		ExitCode:    0,
		ReturnValue: nil,
		GasUsed:     types.ZeroGas,
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
		GasUsed:     types.ZeroGas,
	}
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode) Receipt {
	return Receipt{
		ExitCode:    exitCode,
		ReturnValue: nil,
		GasUsed:     types.ZeroGas,
	}
}

// WithGas sets the gas used.
func (r Receipt) WithGas(amount types.GasUnits) Receipt {
	r.GasUsed = amount
	return r
}
