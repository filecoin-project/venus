package cryptoutil

import (
	"crypto/ecdsa"
	"math/big"
)

//take from "github.com/btcsuite/btcd/btcec"
const (
	PubKeyBytesLenCompressed        = 33
	PubKeyBytesLenUncompressed      = 65
	pubkeyCompressed           byte = 0x2 // y_bit + x coord
	pubkeyUncompressed         byte = 0x4 // x coord + y coord
)

// SerializeCompressed serializes a public key in a 33-byte compressed format.
func SerializeCompressed(p *ecdsa.PublicKey) []byte {
	b := make([]byte, 0, PubKeyBytesLenCompressed)
	format := pubkeyCompressed
	if isOdd(p.Y) {
		format |= 0x1
	}
	b = append(b, format)
	return paddedAppend(32, b, p.X.Bytes())
}

// SerializeUncompressed serializes a public key in a 65-byte uncompressed
// format.
func SerializeUncompressed(p *ecdsa.PublicKey) []byte {
	b := make([]byte, 0, PubKeyBytesLenUncompressed)
	b = append(b, pubkeyUncompressed)
	b = paddedAppend(32, b, p.X.Bytes())
	return paddedAppend(32, b, p.Y.Bytes())
}

// paddedAppend appends the src byte slice to dst, returning the new slice.
// If the length of the source is smaller than the passed size, leading zero
// bytes are appended to the dst slice before appending src.
func paddedAppend(size uint, dst, src []byte) []byte {
	for i := 0; i < int(size)-len(src); i++ {
		dst = append(dst, 0)
	}
	return append(dst, src...)
}

func isOdd(a *big.Int) bool {
	return a.Bit(0) == 1
}
