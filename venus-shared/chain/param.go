package chain

import (
	"math/big"

	big2 "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/chain/params"
)

var (
	bigZero           = big2.Zero()
	blocksPerEpochBig = big.NewInt(0).SetUint64(params.BlocksPerEpoch)
)

var TotalFilecoinInt = FromFil(params.FilBase)

var ZeroAddress = MustParseAddress("f3yaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaby2smx7a")
