package core

import (
	"math/big"

	"github.com/filecoin-project/go-filecoin/types"
)

type Ask struct {
	Price *big.Int
	Size  *big.Int
	Miner types.Address
}
