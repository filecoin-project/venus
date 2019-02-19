package address

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/bls-signatures"
)

// CksmLength is the length of checksum used by Address.
const CksmLength = 6

// Network represents which network an address belongs to.
type Network = uint64

const (
	// Mainnet is the main network.
	Mainnet Network = iota
	// Testnet is the test network.
	Testnet
)

var (
	// MainnetStr is the string representation of Mainnet.
	MainnetStr = "FC"
	// TestnetStr is the string representation of Testnet.
	TestnetStr = "TF"
)

// Protocol represents the type of data address data holds
type Protocol = uint64

const (
	// SECP256K1 means the address is the hash of a secp256k1 public key
	SECP256K1 Protocol = iota
	// ID means the address is an actor ID
	ID
	// Actor means the address is an acotr address, which is a fixed address
	Actor
	// BLS means the address is a full BLS public key
	BLS
)

const (
	// length of address data containing a hash of Secp256k1 public key.
	LengthSecp256k1 = SecpHashLength
	// length of address data containing a hash of actor address.
	LengthActor = SecpHashLength
	// length of address data containing a BLS public key.
	LengthBLS = bls.PublicKeyBytes
)

var (
	// ErrUnknownNetwork is returned when encountering an unknown network in an address.
	ErrUnknownNetwork = errors.New("unknown network")
	// ErrUnknownProtocol is returned when encountering an unknown address type.
	ErrUnknownProtocol = errors.New("unknown protocol")

	// ErrInvalidBytes is returned when encountering an invalid byte format.
	ErrInvalidBytes = errors.New("invalid bytes")
	// ErrInvalidChecksum is returned when encountering an invalid checksum.
	ErrInvalidChecksum = errors.New("invalid checksum")
	// ErrInvalidLength is returned when encountering an invalid length address.
	ErrInvalidLength = errors.New("invalid length")
)

var (
	// TODO Should please stop using this pattern
	// TestAddress is an account with some initial funds in it
	TestAddress Address
	// TODO Should probably stop using this pattern
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
	var err error
	t := Hash([]byte("satoshi"))
	// TODO Should please stop using this pattern
	TestAddress, err = NewFromActor(Mainnet, t)
	if err != nil {
		panic(err)
	}

	t = Hash([]byte("nakamoto"))
	// TODO Should please stop using this pattern
	TestAddress2, err = NewFromActor(Mainnet, t)
	if err != nil {
		panic(err)
	}

	n := Hash([]byte("filecoin"))
	NetworkAddress, err = NewFromActor(Mainnet, n)
	if err != nil {
		panic(err)
	}

	s := Hash([]byte("storage"))
	StorageMarketAddress, err = NewFromActor(Mainnet, s)
	if err != nil {
		panic(err)
	}

	p := Hash([]byte("payments"))
	PaymentBrokerAddress, err = NewFromActor(Mainnet, p)
	if err != nil {
		panic(err)
	}
}
