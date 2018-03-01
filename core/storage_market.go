package core

import (
	"math/big"

	"github.com/filecoin-project/go-filecoin/types"
)

// Ask is a storage market ask order.
type Ask struct {
	Price *big.Int
	Size  *big.Int
	Miner types.Address
	ID    uint64
}
