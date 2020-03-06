package interpreter

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent state.
type VMInterpreter interface {
	// ApplyTipSetMessages applies all the messages in a tipset.
	//
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyTipSetMessages(blocks []BlockMessagesInfo, epoch abi.ChainEpoch, rnd crypto.RandomnessSource) ([]message.Receipt, error)
}

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo struct {
	BLSMessages  []*types.UnsignedMessage
	SECPMessages []*types.SignedMessage
	Miner        address.Address
	TicketCount  int64
}
