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

const EventAMTBitwidth = 5

// MessageReceipt is what is returned by executing a message on the vm.
type MessageReceipt struct {
	version MessageReceiptVersion

	ExitCode exitcode.ExitCode
	Return   []byte
	GasUsed  int64

	EventsRoot *cid.Cid // Root of Event AMT with bitwidth = EventAMTBitwidth
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

func (mr *MessageReceipt) Version() MessageReceiptVersion {
	return mr.version
}

func (mr *MessageReceipt) Equals(o *MessageReceipt) bool {
	return mr.version == o.version && mr.ExitCode == o.ExitCode && bytes.Equal(mr.Return, o.Return) && mr.GasUsed == o.GasUsed &&
		(mr.EventsRoot == o.EventsRoot || (mr.EventsRoot != nil && o.EventsRoot != nil && *mr.EventsRoot == *o.EventsRoot))
}

func (mr *MessageReceipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(mr, "", "  ")
	if err != nil {
		return errStr
	}

	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
