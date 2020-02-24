package message

import (
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	adt_spec "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// EmptyReturnValue is encoded as an empty-list since we use tuple-encoding for everything.
var EmptyReturnValue []byte

func init() {
	out, err := encoding.Encode(&adt_spec.EmptyValue{})
	if err != nil {
		panic(err)
	}
	EmptyReturnValue = out
}

// Receipt is what is returned by executing a message on the vm.
type Receipt struct {
	ExitCode    exitcode.ExitCode `json:"exitCode"`
	ReturnValue []byte            `json:"return"`
	GasUsed     gas.Unit          `json:"gasUsed"`
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
		return Receipt{ExitCode: exitcode.SysErrSerialization}
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
		ReturnValue: EmptyReturnValue,
		GasUsed:     gasAmount,
	}
}

// WithGas sets the gas used.
func (r Receipt) WithGas(amount gas.Unit) Receipt {
	r.GasUsed = amount
	return r
}

func (r *Receipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
