package sectorbuilder

import (
	"bytes"
	"encoding/binary"

	"github.com/filecoin-project/go-filecoin/address"
)

// AddressToProverID creates a prover id by padding an address hash to 31 bytes
func AddressToProverID(addr address.Address) [31]byte {
	// this code will no longer WAE when ID's or BLS pub keys are added
	// as they will break the assumption of addresses all being the same length
	if addr.Protocol() != address.Actor && addr.Protocol() != address.SECP256K1 {
		panic("cannot create prover hash from new address protocol")
	}
	hash := addr.Payload()

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

// SectorIDToBytes encodes the uint64 sector id as a fixed length byte array.
func SectorIDToBytes(sectorID uint64) [31]byte {
	slice := make([]byte, 31)
	binary.LittleEndian.PutUint64(slice, sectorID)

	var sectorIDAsBytes [31]byte
	copy(sectorIDAsBytes[:], slice)

	return sectorIDAsBytes
}
