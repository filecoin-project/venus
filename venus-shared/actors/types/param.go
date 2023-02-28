package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/types/params"
)

var bigZero = big.Zero()

var TotalFilecoinInt = FromFil(params.FilBase)

var ZeroAddress address.Address = One(address.NewFromString("f3yaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaby2smx7a"))

func One[R any](r R, err error) R {
	if err != nil {
		panic(err)
	}

	return r
}
