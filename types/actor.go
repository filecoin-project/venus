package types

import (
	"math/big"

	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(CborEntryFromStruct(Actor{}))
}

type Actor struct {
	Code    *cid.Cid `cbor:"0"`
	Memory  *cid.Cid `cbor:"1"`
	Balance *big.Int `cbor:"2"`
	Nonce   uint64   `cbor:"3"`
}
