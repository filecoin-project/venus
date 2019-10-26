package types

import (
	"encoding/json"
	"fmt"
)

// MessageReceipt represents the result of sending a message.
type MessageReceipt struct {
	// `0` is success, anything else is an error code in unix style.
	ExitCode uint8 `json:"exitCode"`

	// Return contains the return values, if any, from processing a message.
	// This can be non-empty even in the case of error (e.g., to provide
	// programmatically readable detail about errors).
	Return [][]byte `json:"return"`

	// GasAttoFIL Charge is the actual amount of FIL transferred from the sender to the miner for processing the message
	GasAttoFIL AttoFIL `json:"gasAttoFIL"`
}

func (mr *MessageReceipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(mr, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
