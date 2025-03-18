package types

import (
	"math/big"

	big2 "github.com/filecoin-project/go-state-types/big"
)

var EmptyInt = BigInt{}

type BigInt = big2.Int

func NewInt(i uint64) BigInt {
	return BigInt{Int: big.NewInt(0).SetUint64(i)}
}

func BigFromBytes(b []byte) BigInt {
	i := big.NewInt(0).SetBytes(b)
	return BigInt{Int: i}
}

func BigFromString(s string) (BigInt, error) {
	return big2.FromString(s)
}

func BigMul(a, b BigInt) BigInt {
	return big2.Mul(a, b)
}

func BigDiv(a, b BigInt) BigInt {
	return big2.Div(a, b)
}

func BigDivFloat(num, den BigInt) float64 {
	if den.NilOrZero() {
		panic("divide by zero")
	}
	if num.NilOrZero() {
		return 0
	}
	res, _ := new(big.Rat).SetFrac(num.Int, den.Int).Float64()
	return res
}

func BigMod(a, b BigInt) BigInt {
	return big2.Mod(a, b)
}

func BigAdd(a, b BigInt) BigInt {
	return big2.Add(a, b)
}

func BigSub(a, b BigInt) BigInt {
	return big2.Sub(a, b)
}

func BigCmp(a, b BigInt) int {
	return big2.Cmp(a, b)
}
