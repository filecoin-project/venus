package interpreter

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent state.
type VMInterpreter interface {
	// ApplyTipSetMessages applies all the messages in a tipset.
	//
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyTipSetMessages(blocks []BlockMessagesInfo, ts *block.TipSet, parentEpoch abi.ChainEpoch, epoch abi.ChainEpoch) ([]types.MessageReceipt, error)

	// Todo add by force
	ApplyMessage(msg *types.UnsignedMessage) types.MessageReceipt
}

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo struct {
	BLSMessages  []*types.UnsignedMessage
	SECPMessages []*types.SignedMessage
	Miner        address.Address
	WinCount     int64
}
