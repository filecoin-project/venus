package gas

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
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

// AsBigInt returns the internal value as a `big.Int`
func (gas Unit) AsBigInt() big.Int {
	return (big.Int)(gas)
}

// ToTokens returns the cost of the gas given the price.
func (gas Unit) ToTokens(price abi.TokenAmount) abi.TokenAmount {
	// cost = gas * price
	return big.Mul(gas.AsBigInt(), price)
}
