package types

import (
	"github.com/filecoin-project/venus/venus-shared/internal"
)

var EmptyInt = internal.EmptyInt

type BigInt = internal.BigInt

var (
	NewInt        = internal.NewInt
	BigFromBytes  = internal.BigFromBytes
	BigFromString = internal.BigFromString
	BigMul        = internal.BigMul
	BigDiv        = internal.BigDiv
	BigDivFloat   = internal.BigDivFloat
	BigMod        = internal.BigMod
	BigAdd        = internal.BigAdd
	BigSub        = internal.BigSub
	BigCmp        = internal.BigCmp
)
