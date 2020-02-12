package gas

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
)

// Unit is the unit of gas.
type Unit big.Int

// Zero is the zero value for Gas.
var Zero = NewGas(0)

// SystemGasLimit is the maximum gas for implicit system messages.
var SystemGasLimit = NewGas(1000000000000000000) // 10^18

// NewGas creates a gas value object.
func NewGas(value int64) Unit {
	return Unit(big.NewInt(value))
}

// NewLegacyGas is legacy and will be deleted
// Dragons: delete once we finish changing to the new types
func NewLegacyGas(v types.GasUnits) Unit {
	return NewGas((int64)(v))
}

func (gas Unit) asBigInt() big.Int {
	return (big.Int)(gas)
}

// ToTokens returns the cost of the gas given the price.
func (gas Unit) ToTokens(price abi.TokenAmount) abi.TokenAmount {
	// cost = gas * price
	return big.Mul(gas.asBigInt(), price)
}

// Tracker maintains the state of gas usage throughout the execution of a message.
type Tracker struct {
	gasLimit    Unit
	gasConsumed Unit
}

// NewTracker initializes a new empty gas tracker
func NewTracker(limit Unit) Tracker {
	return Tracker{
		gasLimit:    limit,
		gasConsumed: Zero,
	}
}

// Charge will add the gas charge to the current method gas context.
//
// WARNING: this method will panic if there is no sufficient gas left.
func (t *Tracker) Charge(amount Unit) {
	if ok := t.TryCharge(amount); !ok {
		runtime.Abort(exitcode.SysErrOutOfGas)
	}
}

// TryCharge charges `amount` or `RemainingGas()``, whichever is smaller.
//
// Returns `True` if the there was enough gas to pay for `amount`.
func (t *Tracker) TryCharge(amount Unit) bool {
	// check for limit
	aux := big.Add(t.gasConsumed.asBigInt(), amount.asBigInt())
	if aux.GreaterThan(t.gasLimit.asBigInt()) {
		t.gasConsumed = t.gasLimit
		return false
	}

	t.gasConsumed = Unit(aux)
	return true
}

// GasConsumed returns the gas consumed.
func (t Tracker) GasConsumed() Unit {
	return t.gasConsumed
}

// RemainingGas returns the gas remaining.
func (t Tracker) RemainingGas() Unit {
	return Unit(big.Sub(t.gasLimit.asBigInt(), t.gasConsumed.asBigInt()))
}
