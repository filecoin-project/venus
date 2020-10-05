package message

import (
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// Receipt is what is returned by executing a message on the vm.
type Receipt struct {
	// control field for encoding struct as an array
	_           struct{}          `cbor:",toarray"`
	ExitCode    exitcode.ExitCode `json:"exitCode"`
	ReturnValue []byte            `json:"return"`
	GasUsed     gas.Unit          `json:"gasUsed"`
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode, gasAmount gas.Unit) Receipt {
	return Receipt{
		ExitCode:    exitCode,
		ReturnValue: []byte{},
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
