package gas

import (
	"errors"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
)

// Unit is the unit of gas.
type Unit int64

// Zero is the zero value for Gas.
var Zero = NewGas(0)

// SystemGasLimit is the maximum gas for implicit system messages.
var SystemGasLimit = NewGas(1000000000000000000) // 10^18

// NewGas creates a gas value object.
func NewGas(value int64) Unit {
	return Unit(value)
}

// AsBigInt returns the internal value as a `big.Int`
func (gas Unit) AsBigInt() big.Int {
	return big.NewInt(int64(gas))
}

// ToTokens returns the cost of the gas given the price.
func (gas Unit) ToTokens(price abi.TokenAmount) abi.TokenAmount {
	// cost = gas * price
	return big.Mul(gas.AsBigInt(), price)
}

// MarshalBinary ensures that gas.Unit is serialized as a byte array
func (gas *Unit) MarshalBinary() ([]byte, error) {
	bi := big.NewInt(int64(*gas))
	return bi.MarshalBinary()
}

func (gas *Unit) UnmarshalBinary(bs []byte) error {
	bi := big.Zero()
	err := bi.UnmarshalBinary(bs)
	if err != nil {
		return err
	}

	if !bi.IsInt64() {
		return errors.New("serialized units too big for int64")
	}
	*gas = Unit(bi.Int64())
	return nil
}
