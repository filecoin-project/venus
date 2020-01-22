package types

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// NewTestPoSt creates a trivial, right-sized byte slice for a Proof of Spacetime.
func NewTestPoSt() []byte {
	return make([]byte, OnePoStProofPartition.ProofLen())
}

// MockRecoverer implements the Recoverer interface
type MockRecoverer struct{}

// MockSigner implements the Signer interface
type MockSigner struct {
	AddrKeyInfo map[address.Address]KeyInfo
	Addresses   []address.Address
	PubKeys     [][]byte
}

// NewMockSigner returns a new mock signer, capable of signing data with
// keys (addresses derived from) in keyinfo
func NewMockSigner(kis []KeyInfo) MockSigner {
	var ms MockSigner
	ms.AddrKeyInfo = make(map[address.Address]KeyInfo)
	for _, k := range kis {
		// extract public key
		pub := k.PublicKey()

		var newAddr address.Address
		var err error
		if k.CryptSystem == SECP256K1 {
			newAddr, err = address.NewSecp256k1Address(pub)
		} else if k.CryptSystem == BLS {
			newAddr, err = address.NewBLSAddress(pub)
		}
		if err != nil {
			panic(err)
		}
		ms.Addresses = append(ms.Addresses, newAddr)
		ms.AddrKeyInfo[newAddr] = k
		ms.PubKeys = append(ms.PubKeys, pub)
	}
	return ms
}

// NewMockSignersAndKeyInfo is a convenience function to generate a mock
// signers with some keys.
func NewMockSignersAndKeyInfo(numSigners int) (MockSigner, []KeyInfo) {
	ki := MustGenerateKeyInfo(numSigners, 42)
	signer := NewMockSigner(ki)
	return signer, ki
}

// MustGenerateMixedKeyInfo produces m bls keys and n secp keys.
// BLS and Secp will be interleaved. The keys will be valid, but not deterministic.
func MustGenerateMixedKeyInfo(m int, n int) []KeyInfo {
	info := []KeyInfo{}
	for m > 0 && n > 0 {
		if m > 0 {
			pk := bls.PrivateKeyGenerate()
			ki := KeyInfo{
				PrivateKey:  pk[:],
				CryptSystem: BLS,
			}
			info = append(info, ki)
			m--
		}

		if n > 0 {
			pk, err := crypto.GenerateKey()
			if err != nil {
				panic(err)
			}
			ki := KeyInfo{
				PrivateKey:  pk[:],
				CryptSystem: SECP256K1,
			}
			info = append(info, ki)
			n--
		}
	}
	return info
}

// MustGenerateKeyInfo generates `n` distinct keyinfos using seed `seed`.
// The result is deterministic (for stable tests), don't use this for real keys!
func MustGenerateKeyInfo(n int, seed byte) []KeyInfo {
	token := bytes.Repeat([]byte{seed}, 512)
	var keyinfos []KeyInfo
	for i := 0; i < n; i++ {
		token[0] = byte(i)
		prv, err := crypto.GenerateKeyFromSeed(bytes.NewReader(token))
		if err != nil {
			panic(err)
		}

		ki := &KeyInfo{
			PrivateKey:  prv,
			CryptSystem: SECP256K1,
		}
		keyinfos = append(keyinfos, *ki)
	}
	return keyinfos
}

// SignBytes cryptographically signs `data` using the Address `addr`.
func (ms MockSigner) SignBytes(data []byte, addr address.Address) (Signature, error) {
	ki, ok := ms.AddrKeyInfo[addr]
	if !ok {
		return nil, errors.New("Unknown address -- can't sign")
	}

	if ki.CryptSystem == SECP256K1 {
		hash := blake2b.Sum256(data)
		return crypto.SignSecp(ki.Key(), hash[:])
	}
	return crypto.SignBLS(ki.PrivateKey, data)
}

// GetAddressForPubKey looks up a KeyInfo address associated with a given PublicKey for a MockSigner
func (ms MockSigner) GetAddressForPubKey(pk []byte) (address.Address, error) {
	var addr address.Address

	for _, ki := range ms.AddrKeyInfo {
		testPk := ki.PublicKey()

		if bytes.Equal(testPk, pk) {
			addr, err := ki.Address()
			if err != nil {
				return addr, errors.New("could not fetch address")
			}
			return addr, nil
		}
	}
	return addr, errors.New("public key not found in wallet")
}

// NewSignedMessageForTestGetter returns a closure that returns a SignedMessage unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
// TODO support chosing from address
func NewSignedMessageForTestGetter(ms MockSigner) func() *SignedMessage {
	i := 0
	return func() *SignedMessage {
		s := fmt.Sprintf("smsg%d", i)
		i++
		newAddr, err := address.NewSecp256k1Address([]byte(s + "-to"))
		if err != nil {
			panic(err)
		}
		msg := NewMeteredMessage(
			ms.Addresses[0], // from needs to be an address from the signer
			newAddr,
			0,
			ZeroAttoFIL,
			InvalidMethodID,
			[]byte("params"),
			NewGasPrice(0),
			NewGasUnits(0))
		smsg, err := NewSignedMessage(*msg, &ms)
		if err != nil {
			panic(err)
		}
		return smsg
	}
}

// Type-related test helpers.

// CidFromString generates Cid from string input
func CidFromString(t *testing.T, input string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: DefaultHashFunction}
	cid, err := prefix.Sum([]byte(input))
	require.NoError(t, err)
	return cid
}

// NewCidForTestGetter returns a closure that returns a Cid unique to that invocation.
// The Cid is unique wrt the closure returned, not globally. You can use this function
// in tests.
func NewCidForTestGetter() func() cid.Cid {
	i := 31337
	return func() cid.Cid {
		obj, err := cbor.WrapObject([]int{i}, DefaultHashFunction, -1)
		if err != nil {
			panic(err)
		}
		i++
		return obj.Cid()
	}
}

// NewMessageForTestGetter returns a closure that returns a message unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
func NewMessageForTestGetter() func() *UnsignedMessage {
	i := 0
	return func() *UnsignedMessage {
		s := fmt.Sprintf("msg%d", i)
		i++
		from, err := address.NewSecp256k1Address([]byte(s + "-from"))
		if err != nil {
			panic(err)
		}
		to, err := address.NewSecp256k1Address([]byte(s + "-to"))
		if err != nil {
			panic(err)
		}
		return NewUnsignedMessage(
			from,
			to,
			0,
			ZeroAttoFIL,
			MethodID(10000+i),
			nil)
	}
}

// NewMsgs returns n messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewMsgs will return the same
// set of messages).
func NewMsgs(n int) []*UnsignedMessage {
	newMsg := NewMessageForTestGetter()
	msgs := make([]*UnsignedMessage, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
		msgs[i].CallSeqNum = Uint64(i)
	}
	return msgs
}

// NewSignedMsgs returns n signed messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewSignedMsgs will return the same
// set of messages).
func NewSignedMsgs(n uint, ms MockSigner) []*SignedMessage {
	var err error
	newMsg := NewMessageForTestGetter()
	smsgs := make([]*SignedMessage, n)
	for i := uint(0); i < n; i++ {
		msg := newMsg()
		msg.From = ms.Addresses[0]
		msg.CallSeqNum = Uint64(i)
		msg.GasPrice = NewGasPrice(1)
		msg.GasLimit = NewGasUnits(0)
		smsgs[i], err = NewSignedMessage(*msg, ms)
		if err != nil {
			panic(err)
		}
	}
	return smsgs
}

// SignMsgs returns a slice of signed messages where the original messages
// are `msgs`, if signing one of the `msgs` fails an error is returned
func SignMsgs(ms MockSigner, msgs []*UnsignedMessage) ([]*SignedMessage, error) {
	var smsgs []*SignedMessage
	for _, m := range msgs {
		s, err := NewSignedMessage(*m, &ms)
		if err != nil {
			return nil, err
		}
		smsgs = append(smsgs, s)
	}
	return smsgs, nil
}

// MsgCidsEqual returns true if the message cids are equal. It panics if
// it can't get their cid.
func MsgCidsEqual(m1, m2 *UnsignedMessage) bool {
	m1Cid, err := m1.Cid()
	if err != nil {
		panic(err)
	}
	m2Cid, err := m2.Cid()
	if err != nil {
		panic(err)
	}
	return m1Cid.Equals(m2Cid)
}

// SmsgCidsEqual returns true if the SignedMessage cids are equal. It panics if
// it can't get their cid.
func SmsgCidsEqual(m1, m2 *SignedMessage) bool {
	m1Cid, err := m1.Cid()
	if err != nil {
		panic(err)
	}
	m2Cid, err := m2.Cid()
	if err != nil {
		panic(err)
	}
	return m1Cid.Equals(m2Cid)
}

// NewMsgsWithAddrs returns a slice of `n` messages who's `From` field's are pulled
// from `a`. This method should be used when the addresses returned are to be signed
// at a later point.
func NewMsgsWithAddrs(n int, a []address.Address) []*UnsignedMessage {
	if n > len(a) {
		panic("cannot create more messages than there are addresess for")
	}
	newMsg := NewMessageForTestGetter()
	msgs := make([]*UnsignedMessage, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
		msgs[i].From = a[i]
	}
	return msgs
}

// HasCid allows two values with CIDs to be compared.
type HasCid interface {
	Cid() cid.Cid
}

// AssertHaveSameCid asserts that two values have identical CIDs.
func AssertHaveSameCid(t *testing.T, m HasCid, n HasCid) {
	if !m.Cid().Equals(n.Cid()) {
		assert.Fail(t, "CIDs don't match", "not equal %v %v", m.Cid(), n.Cid())
	}
}

// AssertCidsEqual asserts that two CIDS are identical.
func AssertCidsEqual(t *testing.T, m cid.Cid, n cid.Cid) {
	if !m.Equals(n) {
		assert.Fail(t, "CIDs don't match", "not equal %v %v", m, n)
	}
}
