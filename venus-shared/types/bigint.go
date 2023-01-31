package types

import (
	"github.com/filecoin-project/venus/venus-shared/actors/types"
)

var EmptyInt = types.EmptyInt

type BigInt = types.BigInt

var (
	NewInt        = types.NewInt
	BigFromBytes  = types.BigFromBytes
	BigFromString = types.BigFromString
	BigMul        = types.BigMul
	BigDiv        = types.BigDiv
	BigDivFloat   = types.BigDivFloat
	BigMod        = types.BigMod
	BigAdd        = types.BigAdd
	BigSub        = types.BigSub
	BigCmp        = types.BigCmp
)
