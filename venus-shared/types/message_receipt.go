package types

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/ipfs/go-cid"
)

type MessageReceiptVersion byte

const (
	// MessageReceiptV0 refers to pre FIP-0049 receipts.
	MessageReceiptV0 MessageReceiptVersion = 0
	// MessageReceiptV1 refers to post FIP-0049 receipts.
	MessageReceiptV1 MessageReceiptVersion = 1
)

// MessageReceipt is what is returned by executing a message on the vm.
type MessageReceipt struct {
	version MessageReceiptVersion

	ExitCode exitcode.ExitCode
	Return   []byte
	GasUsed  int64

	EventsRoot *cid.Cid // Root of Event AMT
}

// NewMessageReceiptV0 creates a new pre FIP-0049 receipt with no capability to
// convey events.
func NewMessageReceiptV0(exitcode exitcode.ExitCode, ret []byte, gasUsed int64) MessageReceipt {
	return MessageReceipt{
		version:  MessageReceiptV0,
		ExitCode: exitcode,
		Return:   ret,
		GasUsed:  gasUsed,
	}
}

// NewMessageReceiptV1 creates a new pre FIP-0049 receipt with the ability to
// convey events.
func NewMessageReceiptV1(exitcode exitcode.ExitCode, ret []byte, gasUsed int64, eventsRoot *cid.Cid) MessageReceipt {
	return MessageReceipt{
		version:    MessageReceiptV1,
		ExitCode:   exitcode,
		Return:     ret,
		GasUsed:    gasUsed,
		EventsRoot: eventsRoot,
	}
}

func (r *MessageReceipt) Version() MessageReceiptVersion {
	return r.version
}

func (r *MessageReceipt) Equals(o *MessageReceipt) bool {
	return r.version == o.version && r.ExitCode == o.ExitCode && bytes.Equal(r.Return, o.Return) && r.GasUsed == o.GasUsed &&
		(r.EventsRoot == o.EventsRoot || (r.EventsRoot != nil && o.EventsRoot != nil && *r.EventsRoot == *o.EventsRoot))
}

func (r *MessageReceipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errStr
	}

	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
