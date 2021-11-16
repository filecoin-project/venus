package chain

import (
	"math/big"

	"github.com/filecoin-project/venus/venus-shared/chain/params"
)

var blocksPerEpochBig = big.NewInt(0).SetUint64(params.BlocksPerEpoch)
