package sectorbuilder

import (
	"bytes"
	"encoding/binary"

	"github.com/filecoin-project/go-filecoin/address"
)

// AddressToProverID creates a prover id by padding an address hash to 31 bytes
func AddressToProverID(addr address.Address) [31]byte {
	hash := address.Hash(addr.Bytes())

	dlen := 31          // desired length
	hlen := len(hash)   // hash length
	padl := dlen - hlen // padding length

	var prid [31]byte

	// will copy dlen bytes from hash
	copy(prid[:], hash)

	if padl > 0 {
		copy(prid[hlen:], bytes.Repeat([]byte{0}, padl))
	}

	return prid
}

// SectorIDToBytes creates a prover id by padding an address hash to 31 bytes
func SectorIDToBytes(sectorID uint64) [31]byte {
	slice := make([]byte, 31)
	binary.LittleEndian.PutUint64(slice, sectorID)

	var sectorIDAsBytes [31]byte
	copy(sectorIDAsBytes[:], slice)

	return sectorIDAsBytes
}
