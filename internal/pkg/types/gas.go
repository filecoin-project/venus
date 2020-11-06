package types

import (
	"github.com/filecoin-project/go-state-types/big"
)

// Unit is the unit of gas.
// This type is signed by design; it is possible for operations to consume negative gas.
type Unit int64

// Zero is the zero value for Gas.
var Zero = NewGas(0)

// SystemGasLimit is the maximum gas for implicit system messages.
var SystemGasLimit = NewGas(10_000_000_000 * 10000) // 10^18

// NewGas creates a gas value object.
func NewGas(value int64) Unit {
	return Unit(value)
}

// AsBigInt returns the internal value as a `big.Int`
func (gas Unit) AsBigInt() big.Int {
	return big.NewInt(int64(gas))
}
