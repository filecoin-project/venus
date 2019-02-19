package address

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"fmt"

	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/bls-signatures"
)

func init() {
	cbor.RegisterCborType(Address{})
}

// newAddress returns an address for network `n`, for protocol `p`, containing value `data`.
func newAddress(n Network, p Protocol, data []byte) (Address, error) {
	// check for valid inputs
	if n != Mainnet && n != Testnet {
		return Undef, ErrUnknownNetwork
	}
	if p != SECP256K1 && p != ID && p != Actor && p != BLS {
		return Undef, ErrUnknownProtocol

	}
	datalen := len(data)
	// two 8 bytes (max) numbers plus data
	buf := make([]byte, 2*binary.MaxVarintLen64+datalen)
	c := binary.PutUvarint(buf, n)
	c += binary.PutUvarint(buf[c:], p)
	cn := copy(buf[c:], data)
	if cn != datalen {
		panic("copy data length is inconsistent")
	}

	newAddr := Address{string(buf[:c+datalen])}
	return newAddr, nil
}

// Address is the go type that represents an address in the filecoin network.
type Address struct{ str string }

var Undef = Address{}

// Network represents which network the address belongs to.
func (a Address) Network() Network {
	network, _ := uvarint(a.str)
	return network
}

// Protocol represents the type of content contained in the address.
func (a Address) Protocol() Protocol {
	_, c := uvarint(a.str)
	protocol, _ := uvarint(a.str[c:])
	return protocol
}

// Data represents the content contained in the address.
func (a Address) Data() []byte {
	_, c := uvarint(a.str)
	_, cn := uvarint(a.str)
	return []byte(a.str[c+cn:])
}

// Empty returns true if the address is empty.
func (a Address) Empty() bool {
	return 0 == len(a.str)
}

// Bytes returns the byte representation of the address.
func (a Address) Bytes() []byte {
	return []byte(a.str)
}

func (a Address) String() string {
	return a.encode()
}

func (a Address) Format(f fmt.State, c rune) {
	switch c {
	case 'v':
		fmt.Fprintf(f, "[%s - %x - %x]", NetworkToString(a.Network()), a.Protocol(), a.Data())
	case 's':
		fmt.Fprintf(f, "%s", a.String())
	default:
		fmt.Fprintf(f, "%"+string(c), a.Bytes())
	}
}

// Encode returns the string representation of an Address, or `<invalid address>` on a best effort basis if `a` is invalid.
func (a Address) encode() string {
	if len(a.str) < 3 {
		return "<invalid address>"
	}

	var prefix string
	switch a.Network() {
	case Mainnet:
		prefix = MainnetStr
		break
	case Testnet:
		prefix = TestnetStr
		break
	default:
		return "<invalid address>"
	}

	// ID's do not have a checksum
	if a.Protocol() == ID {
		return prefix + base32.StdEncoding.EncodeToString(a.suffix())
	}

	return prefix + base32.StdEncoding.EncodeToString(append(a.suffix(), checksum(a.suffix())...))
}

// decodeFromString is a helper method that attempts to extract the components from an address `addr`.
func decodeFromString(addr string) (string, Protocol, []byte, []byte, error) {
	if len(addr) < 4 {
		return "", 0, []byte{}, []byte{}, ErrInvalidLength
	}

	var ntwrk string
	switch addr[:2] {
	case MainnetStr:
		ntwrk = MainnetStr
		break
	case TestnetStr:
		ntwrk = TestnetStr
		break
	default:
		return "", 0, []byte{}, []byte{}, ErrUnknownNetwork
	}

	// Decode the `Protocol` and associated `data`
	raw, err := base32.StdEncoding.DecodeString(addr[2:])
	if err != nil {
		return "", 0, []byte{}, []byte{}, err
	}
	// `Protocol` is before data and is uvarin encoded
	protocol, protolen := binary.Uvarint(raw)

	// if the `Protocol` is an ID there is no checksum after data
	cksmOffset := CksmLength
	if protocol == ID {
		cksmOffset = 0
	}

	// data is everything after protocol up to the last CksmLength bytes
	data := raw[protolen : len(raw)-cksmOffset]
	// checksum is the last CksmLength bytes
	cksm := raw[len(raw)-cksmOffset : len(raw)]

	return ntwrk, protocol, data, cksm, nil
}

// NewFromString tries to parse a given string into a filecoin address.
func NewFromString(addr string) (Address, error) {
	// decode the address into is components
	ntwrk, proto, data, cksm, err := decodeFromString(addr)
	if err != nil {
		return Undef, err
	}

	// does the address belong to a known network
	var network Network
	switch ntwrk {
	case MainnetStr:
		network = Mainnet
	case TestnetStr:
		network = Testnet
	default:
		return Undef, ErrUnknownNetwork
	}

	networkBuf := make([]byte, binary.MaxVarintLen64)
	cn := binary.PutUvarint(networkBuf, network)
	networkBuf = networkBuf[:cn]

	protoBuf := make([]byte, binary.MaxVarintLen64)
	cp := binary.PutUvarint(protoBuf, proto)
	protoBuf = protoBuf[:cp]

	// the checksum digest is produced from the address type and data, so grab that here.
	cksmIngest := append(protoBuf, data...)
	switch proto {
	case SECP256K1:
		if len(data) != LengthSecp256k1 {
			return Undef, ErrInvalidBytes
		}
		if !verifyChecksum(cksmIngest, cksm) {
			return Undef, ErrInvalidChecksum
		}
		break
	case ID:
		if len(data) < 1 {
			return Undef, ErrInvalidBytes
		}
		// ID's do not have a checksum
		break
	case Actor:
		if len(data) != LengthActor {
			return Undef, ErrInvalidBytes
		}
		if !verifyChecksum(cksmIngest, cksm) {
			return Undef, ErrInvalidChecksum
		}
		break
	case BLS:
		if len(data) != LengthBLS {
			return Undef, ErrInvalidBytes
		}
		if !verifyChecksum(cksmIngest, cksm) {
			return Undef, ErrInvalidChecksum
		}
		break
	default:
		return Undef, ErrUnknownProtocol
	}

	var raw []byte
	// add the network bytes
	raw = append(raw, networkBuf...)
	// add the protocol bytes
	raw = append(raw, protoBuf...)
	// add the data
	raw = append(raw, data...)
	return Address{string(raw)}, nil
}

func NewFromBytes(raw []byte) (Address, error) {
	network, c1 := binary.Uvarint(raw)
	protocol, c2 := binary.Uvarint(raw[c1:])
	data := raw[c1+c2:]
	return newAddress(network, protocol, data)
}

// NewFromBLS returns an address for the actor represented by BLS public key `pk`.
func NewFromBLS(n Network, pk bls.PublicKey) (Address, error) {
	return newAddress(n, BLS, pk[:])
}

// NewFromActorID returns an address for the actor represented by `id`.
func NewFromActorID(n Network, id uint64) (Address, error) {
	buf := make([]byte, binary.MaxVarintLen64)
	c := binary.PutUvarint(buf, id)
	buf = buf[:c]
	return newAddress(n, ID, buf)
}

// NewFromActor returns an address created for the actor represented by `data`.
func NewFromActor(n Network, hofdata []byte) (Address, error) {
	return newAddress(n, Actor, hofdata)
}

// NetworkToString creates a human readable representation of the network.
func NetworkToString(n Network) string {
	switch n {
	case Mainnet:
		return MainnetStr
	case Testnet:
		return TestnetStr
	default:
		panic("invalid network")
	}
}

// checksum returns the last CksmLength bytes of double sha256 data.
func checksum(data []byte) []byte {
	cksm := sha256.Sum256(data)
	cksm = sha256.Sum256(cksm[:])
	return cksm[len(cksm)-CksmLength:]
}

// verify checksum returns true if checksum(data) matches cksm
func verifyChecksum(data, cksm []byte) bool {
	maybeCksm := sha256.Sum256(data)
	maybeCksm = sha256.Sum256(maybeCksm[:])
	return (0 == bytes.Compare(maybeCksm[len(maybeCksm)-CksmLength:], cksm))
}

// Address sufix is everything encoded on an address -- the protocol and its data.
func (a Address) suffix() []byte {
	// since the Network isn't encoded, take all but that
	_, n := uvarint(a.str)
	return []byte(a.str[n:])
}
