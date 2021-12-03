package chain

import (
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/exitcode"
)

// MessageReceipt is what is returned by executing a message on the vm.
type MessageReceipt struct {
	ExitCode exitcode.ExitCode
	Return   []byte
	GasUsed  int64
}

func (r *MessageReceipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errStr
	}

	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
