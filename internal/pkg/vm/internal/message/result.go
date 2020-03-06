package message

import (
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// Receipt is what is returned by executing a message on the vm.
type Receipt struct {
	ExitCode    exitcode.ExitCode `json:"exitCode"`
	ReturnValue []byte            `json:"return"`
	GasUsed     gas.Unit          `json:"gasUsed"`
}

// Value returns a successful code with the value encoded.
//
// Callers do NOT need to encode the value before calling this method.
func Value(obj interface{}, gasUsed gas.Unit) Receipt {
	code := exitcode.Ok
	var aux []byte
	if obj != nil {
		var err error
		aux, err = encoding.Encode(obj)
		if err != nil {
			code = exitcode.SysErrSerialization
		}
	}

	return Receipt{
		ExitCode:    code,
		ReturnValue: aux,
		GasUsed:     gasUsed,
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

func (r *Receipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
