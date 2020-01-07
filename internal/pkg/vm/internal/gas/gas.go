package gas

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
)

// Unit is the unit of gas.
// Dragons: move the gasunits to be defined here.
type Unit = types.GasUnits

// SystemGasLimit is the maximum gas for implicit system messages.
var SystemGasLimit = (Unit)(types.NewGasUnits(10 ^ 18))

// Tracker maintains the state of gas usage throughout the execution of a message.
type Tracker struct {
	gasLimit    Unit
	gasConsumed Unit
}

// NewTracker initializes a new empty gas tracker
func NewTracker(limit Unit) Tracker {
	return Tracker{
		gasLimit:    limit,
		gasConsumed: types.ZeroGas,
	}
}

// Charge will add the gas charge to the current method gas context.
//
// WARNING: this method will panic if there is no sufficient gas left.
func (t *Tracker) Charge(amount Unit) {
	if ok := t.TryCharge(amount); !ok {
		panic(exitcode.OutOfGas)
	}
}

// TryCharge will add the gas charge to the current method gas context.
//
// This method returns `True` if the there was enough gas.
//
// Note: this method will Zero out the gas when it returns `False`.
func (t *Tracker) TryCharge(amount Unit) bool {
	// check for limit
	if t.gasConsumed+amount > t.gasLimit {
		t.gasConsumed = t.gasLimit
		return false
	}

	t.gasConsumed += amount
	return true
}

// GasConsumed returns the gas consumed.
func (t Tracker) GasConsumed() Unit {
	return t.gasConsumed
}

// RemainingGas returns the gas remaining.
func (t Tracker) RemainingGas() Unit {
	return t.gasLimit - t.gasConsumed
}
