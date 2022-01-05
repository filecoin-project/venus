package internal

import (
	"github.com/filecoin-project/go-address"

	big2 "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/types/params"
)

var (
	bigZero = big2.Zero()
)

var TotalFilecoinInt = FromFil(params.FilBase)

var ZeroAddress = func() address.Address {
	addr := "f3yaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaby2smx7a"

	ret, err := address.NewFromString(addr)
	if err != nil {
		panic(err)
	}

	return ret
}()
