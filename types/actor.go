package types

import (
	"math/big"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

type Actor struct {
	Code    *cid.Cid
	Memory  *cid.Cid
	Balance *big.Int
	Nonce   uint64
}
