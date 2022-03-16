package market

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type SignInfo struct {
	Data interface{}
	Type types.MsgType
	Addr address.Address
}

type User struct {
	Addr    address.Address
	Account string
}

type MarketBalance struct { //nolint
	Escrow big.Int
	Locked big.Int
}
