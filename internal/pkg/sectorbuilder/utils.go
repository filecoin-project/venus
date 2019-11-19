package sectorbuilder

import (
	"bytes"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// AddressToProverID creates a prover id by padding an address hash to 31 bytes
func AddressToProverID(addr address.Address) [31]byte {
	payload := addr.Payload()
	if len(payload) > 31 {
		panic("cannot create prover id from address because address is too long")
	}

	dlen := 31           // desired length
	hlen := len(payload) // hash length
	padl := dlen - hlen  // padding length

	var prid [31]byte

	// will copy dlen bytes from hash
	copy(prid[:], payload)

	if padl > 0 {
		copy(prid[hlen:], bytes.Repeat([]byte{0}, padl))
	}

	return prid
}
