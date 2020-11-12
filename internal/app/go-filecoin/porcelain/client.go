package porcelain

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/internal/pkg/types"
)

// Ask is a result of querying for an ask, it may contain an error
type Ask struct {
	Miner  address.Address
	Price  types.AttoFIL
	Expiry abi.ChainEpoch
	ID     uint64

	Error error
}
