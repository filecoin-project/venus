package gastracker

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
)

// GasTracker maintains the state of gas usage throughout the execution of a block and a message
type GasTracker struct {
	MsgGasLimit          types.GasUnits
	gasConsumedByBlock   types.GasUnits
	gasConsumedByMessage types.GasUnits
}

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker() *GasTracker {
	return &GasTracker{
		MsgGasLimit:          types.NewGasUnits(0),
		gasConsumedByBlock:   types.NewGasUnits(0),
		gasConsumedByMessage: types.NewGasUnits(0),
	}
}

// ResetForNewMessage will reset the per-message gas accumulator and set the MsgGasLimit to that of the message.
func (gasTracker *GasTracker) ResetForNewMessage(message *types.UnsignedMessage) {
	gasTracker.MsgGasLimit = message.GasLimit
	gasTracker.gasConsumedByMessage = types.NewGasUnits(0)
}

// Charge will add the gas charge to the current method gas context.
func (gasTracker *GasTracker) Charge(cost types.GasUnits) error {
	if gasTracker.gasConsumedByMessage+cost > gasTracker.MsgGasLimit {
		gasTracker.gasConsumedByMessage = gasTracker.MsgGasLimit
		gasTracker.gasConsumedByBlock += gasTracker.MsgGasLimit
		return errors.NewRevertError("gas cost exceeds gas limit")
	}

	gasTracker.gasConsumedByMessage += cost
	gasTracker.gasConsumedByBlock += cost
	return nil
}

// GasAboveBlockLimit will return true if the MsgGasLimit of the current message is greater than the block gas limit.
func (gasTracker *GasTracker) GasAboveBlockLimit() bool {
	return gasTracker.MsgGasLimit > types.BlockGasLimit
}

// GasTooHighForCurrentBlock will return true if the MsgGasLimit of the current message
// plus the gas used for the current block is greater than the block gas limit.
func (gasTracker *GasTracker) GasTooHighForCurrentBlock() bool {
	return gasTracker.MsgGasLimit+gasTracker.gasConsumedByBlock > types.BlockGasLimit
}

// GasConsumedByMessage returns the gas consumed by the message.
func (gasTracker *GasTracker) GasConsumedByMessage() types.GasUnits {
	return gasTracker.gasConsumedByMessage
}
