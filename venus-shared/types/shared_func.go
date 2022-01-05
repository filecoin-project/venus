package types

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

func MustParseAddress(addr string) address.Address {
	ret, err := address.NewFromString(addr)
	if err != nil {
		panic(err)
	}

	return ret
}

func MustParseCid(c string) cid.Cid {
	ret, err := cid.Decode(c)
	if err != nil {
		panic(err)
	}

	return ret
}

func NewGasFeeCap(price int64) abi.TokenAmount {
	return abi.NewTokenAmount(price)
}

func NewGasPremium(price int64) abi.TokenAmount {
	return abi.NewTokenAmount(price)
}
