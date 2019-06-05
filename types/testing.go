package types

import (
	"bytes"
	"fmt"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	wutil "github.com/filecoin-project/go-filecoin/wallet/util"
)

// NewTestPoSt creates a trivial, right-sized byte slice for a Proof of Spacetime.
func NewTestPoSt() []byte {
	return make([]byte, OnePoStProofPartition.ProofLen())
}

// MockRecoverer implements the Recoverer interface
type MockRecoverer struct{}

// Ecrecover returns an uncompressed public key that could produce the given
// signature from data.
// Note: The returned public key should not be used to verify `data` is valid
// since a public key may have N private key pairs
func (mr *MockRecoverer) Ecrecover(data []byte, sig Signature) ([]byte, error) {
	return wutil.Ecrecover(data, sig)
}

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
		newAddr, err := address.NewSecp256k1Address(pub)
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
	ki := MustGenerateKeyInfo(numSigners, GenerateKeyInfoSeed())
	signer := NewMockSigner(ki)
	return signer, ki
}

// SignBytes cryptographically signs `data` using the Address `addr`.
func (ms MockSigner) SignBytes(data []byte, addr address.Address) (Signature, error) {
	ki, ok := ms.AddrKeyInfo[addr]
	if !ok {
		panic("unknown address")
	}

	hash := blake2b.Sum256(data)
	return crypto.Sign(ki.Key(), hash[:])
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

// CreateTicket is effectively a duplicate of Wallet CreateTicket for testing purposes.
func (ms MockSigner) CreateTicket(proof PoStProof, signerPubKey []byte) (Signature, error) {
	var ticket Signature

	signerAddr, err := ms.GetAddressForPubKey(signerPubKey)
	if err != nil {
		return ticket, err
	}

	buf := append(proof[:], signerAddr.Bytes()...)
	h := blake2b.Sum256(buf)

	ticket, err = ms.SignBytes(h[:], signerAddr)
	if err != nil {
		errMsg := fmt.Sprintf("SignBytes error in CreateTicket: %s", err.Error())
		panic(errMsg)
	}
	return ticket, nil
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
		newAddr, err := address.NewActorAddress([]byte(s + "-to"))
		if err != nil {
			panic(err)
		}
		msg := NewMessage(
			ms.Addresses[0], // from needs to be an address from the signer
			newAddr,
			0,
			NewAttoFILFromFIL(0),
			s,
			[]byte("params"))
		smsg, err := NewSignedMessage(*msg, &ms, NewGasPrice(0), NewGasUnits(0))
		if err != nil {
			panic(err)
		}
		return smsg
	}
}

// Type-related test helpers.

// SomeCid generates a Cid for use in tests where you want a Cid but don't care
// what it is.
func SomeCid() cid.Cid {
	b := &Block{}
	return b.Cid()
}

// NewCidForTestGetter returns a closure that returns a Cid unique to that invocation.
// The Cid is unique wrt the closure returned, not globally. You can use this function
// in tests.
func NewCidForTestGetter() func() cid.Cid {
	i := Uint64(31337)
	return func() cid.Cid {
		b := &Block{Height: i}
		i++
		return b.Cid()
	}
}

// NewMessageForTestGetter returns a closure that returns a message unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
func NewMessageForTestGetter() func() *Message {
	i := 0
	return func() *Message {
		s := fmt.Sprintf("msg%d", i)
		i++
		from, err := address.NewActorAddress([]byte(s + "-from"))
		if err != nil {
			panic(err)
		}
		to, err := address.NewActorAddress([]byte(s + "-to"))
		if err != nil {
			panic(err)
		}
		return NewMessage(
			from,
			to,
			0,
			nil,
			s,
			nil)
	}
}

// NewBlockForTest returns a new block. If a parent block is provided, the returned
// block will be configured as if it were a child of that parent. The returned block
// has not been persisted into the store.
func NewBlockForTest(parent *Block, nonce uint64) *Block {
	block := &Block{
		Nonce:           Uint64(nonce),
		Messages:        []*SignedMessage{},
		MessageReceipts: []*MessageReceipt{},
	}

	if parent != nil {
		block.Height = parent.Height + 1
		block.StateRoot = parent.StateRoot
		block.Parents.Add(parent.Cid())
	}

	return block
}

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(t *testing.T, blks ...*Block) TipSet {
	ts, err := NewTipSet(blks...)
	require.NoError(t, err)
	return ts
}

// NewMsgs returns n messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewMsgs will return the same
// set of messages).
func NewMsgs(n int) []*Message {
	newMsg := NewMessageForTestGetter()
	msgs := make([]*Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
		msgs[i].Nonce = Uint64(i)
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
		msg.Nonce = Uint64(i)
		smsgs[i], err = NewSignedMessage(*msg, ms, NewGasPrice(1), NewGasUnits(0))
		if err != nil {
			panic(err)
		}
	}
	return smsgs
}

// SignMsgs returns a slice of signed messages where the original messages
// are `msgs`, if signing one of the `msgs` fails an error is returned
func SignMsgs(ms MockSigner, msgs []*Message) ([]*SignedMessage, error) {
	var smsgs []*SignedMessage
	for _, m := range msgs {
		s, err := NewSignedMessage(*m, &ms, NewGasPrice(0), NewGasUnits(0))
		if err != nil {
			return nil, err
		}
		smsgs = append(smsgs, s)
	}
	return smsgs, nil
}

// MsgCidsEqual returns true if the message cids are equal. It panics if
// it can't get their cid.
func MsgCidsEqual(m1, m2 *Message) bool {
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
func NewMsgsWithAddrs(n int, a []address.Address) []*Message {
	if n > len(a) {
		panic("cannot create more messages than there are addresess for")
	}
	newMsg := NewMessageForTestGetter()
	msgs := make([]*Message, n)
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
