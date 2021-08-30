package types

import "github.com/filecoin-project/go-state-types/big"

type BlockReward struct {
	Block   *BlockHeader
	Rewards big.Int
	Penalty big.Int
}
