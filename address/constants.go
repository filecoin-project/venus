package address

import (
	"github.com/dignifiedquire/go-basex"
)

// HashLength is the length of an the hash part of the address in bytes.
const HashLength = 20

// Length is the lengh of a full address in bytes.
const Length = 1 + 1 + HashLength

// Version is the current version of the address format.
const Version byte = 0

// Base32Charset is the character set used for base32 encoding in addresses.
const Base32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// Base32CharsetReverse is the reverse character set. It maps ASCII byte -> Base32Charset index on [0,31].
var Base32CharsetReverse = [128]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	15, -1, 10, 17, 21, 20, 26, 30, 7, 5, -1, -1, -1, -1, -1, -1,
	-1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1,
	1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
	-1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1,
	1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
}

// Base32 is a basex instance using the Base32Charset.
var Base32 = basex.NewAlphabet(Base32Charset)

var (
	// TestAddress is an account with some initial funds in it
	TestAddress Address
	// TestAddress2 is an account with some initial funds in it
	TestAddress2 Address
	// NetworkAddress is the filecoin network
	NetworkAddress Address
	// StorageMarketAddress is the hard-coded address of the filecoin storage market
	StorageMarketAddress Address
	// PaymentBrokerAddress is the hard-coded address of the filecoin storage market
	PaymentBrokerAddress Address
)

func init() {
	t := Hash([]byte("satoshi"))
	TestAddress = NewMainnet(t)

	t = Hash([]byte("nakamoto"))
	TestAddress2 = NewMainnet(t)

	n := Hash([]byte("filecoin"))
	NetworkAddress = NewMainnet(n)

	s := Hash([]byte("storage"))
	StorageMarketAddress = NewMainnet(s)

	p := Hash([]byte("payments"))
	PaymentBrokerAddress = NewMainnet(p)
}
