package interpreter

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/specs-actors/actors/abi"
)

// VMInterpreter orchestrates the execution of messages from a tipset on that tipset’s parent state.
type VMInterpreter interface {
	// ApplyTipSetMessages applies all the messages in a tipset.
	//
	// Note: any message processing error will be present as an `ExitCode` in the `MessageReceipt`.
	ApplyTipSetMessages(rnd RandomnessSource, msgs []BlockMessagesInfo, epoch abi.ChainEpoch) ([]message.Receipt, error)
}

// RandomnessSource provides randomness to actors.
type RandomnessSource interface {
	Randomness(epoch abi.ChainEpoch) abi.Randomness
}

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo struct {
	BLSMessages  []*types.UnsignedMessage
	SECPMessages []*types.SignedMessage
	Miner        address.Address
}
