package types

import (
	"crypto/ecdsa"
	"fmt"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	cu "github.com/filecoin-project/go-filecoin/crypto/util"
	wutil "github.com/filecoin-project/go-filecoin/wallet/util"
)

// NewTestPoSt creates a trivial, right-sized byte slice for a Proof of Spacetime.
func NewTestPoSt() [192]byte {
	var newProof [192]byte
	return newProof
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
}

// NewMockSigner returns a new mock signer, capable of signing data with
// keys (addresses derived from) in keyinfo
func NewMockSigner(kis []KeyInfo) MockSigner {
	var ms MockSigner
	ms.AddrKeyInfo = make(map[address.Address]KeyInfo)
	for _, k := range kis {
		// get the secret key
		sk, err := crypto.BytesToECDSA(k.PrivateKey)
		if err != nil {
			panic(err)
		}
		// extract public key
		pub, ok := sk.Public().(*ecdsa.PublicKey)
		if !ok {
			panic("unknown public key type")
		}
		addrHash := address.Hash(cu.SerializeUncompressed(pub))
		newAddr := address.NewMainnet(addrHash)
		ms.Addresses = append(ms.Addresses, newAddr)
		ms.AddrKeyInfo[newAddr] = k

	}
	return ms
}

// SignBytes cryptographically signs `data` using the Address `addr`.
func (ms MockSigner) SignBytes(data []byte, addr address.Address) (Signature, error) {
	ki, ok := ms.AddrKeyInfo[addr]
	if !ok {
		panic("unknown address")
	}

	sk, err := crypto.BytesToECDSA(ki.PrivateKey)
	if err != nil {
		return Signature{}, err
	}

	return wutil.Sign(sk, data)
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
		msg := NewMessage(
			ms.Addresses[0], // from needs to be an address from the signer
			address.NewMainnet([]byte(s+"-to")),
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
		return NewMessage(
			address.NewMainnet([]byte(s+"-from")),
			address.NewMainnet([]byte(s+"-to")),
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
func RequireNewTipSet(require *require.Assertions, blks ...*Block) TipSet {
	ts, err := NewTipSet(blks...)
	require.NoError(err)
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
	}
	return msgs
}

// NewSignedMsgs returns n signed messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewSignedMsgs will return the same
// set of messages).
func NewSignedMsgs(n int, ms MockSigner) []*SignedMessage {
	newSmsg := NewSignedMessageForTestGetter(ms)
	smsgs := make([]*SignedMessage, n)
	for i := 0; i < n; i++ {
		smsgs[i] = newSmsg()
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
func AssertHaveSameCid(a *assert.Assertions, m HasCid, n HasCid) {
	if !m.Cid().Equals(n.Cid()) {
		a.Fail("CIDs don't match", "not equal %v %v", m.Cid(), n.Cid())
	}
}

// AssertCidsEqual asserts that two CIDS are identical.
func AssertCidsEqual(a *assert.Assertions, m cid.Cid, n cid.Cid) {
	if !m.Equals(n) {
		a.Fail("CIDs don't match", "not equal %v %v", m, n)
	}
}
