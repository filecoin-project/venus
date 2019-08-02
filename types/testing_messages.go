package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/filecoin-project/go-filecoin/address"
)

// MessageMaker creates unique, signed messages for use in tests.
type MessageMaker struct {
	DefaultGasPrice AttoFIL
	DefaultGasUnits GasUnits

	signer *MockSigner
	seq    uint
	t      *testing.T
}

// NewMessageMaker creates a new message maker with a set of signing keys.
func NewMessageMaker(t *testing.T, keys []KeyInfo) *MessageMaker {
	addresses := make([]address.Address, len(keys))
	signer := NewMockSigner(keys)

	for i, key := range keys {
		addr, _ := key.Address()
		addresses[i] = addr
	}

	return &MessageMaker{ZeroAttoFIL, NewGasUnits(0), &signer, 0, t}
}

// Addresses returns the addresses for which this maker can sign messages.
func (mm *MessageMaker) Addresses() []address.Address {
	return mm.signer.Addresses
}

// Signer returns the signer with which this maker signs messages.
func (mm *MessageMaker) Signer() *MockSigner {
	return mm.signer
}

// NewSignedMessage creates a new message.
func (mm *MessageMaker) NewSignedMessage(from address.Address, nonce uint64) *SignedMessage {
	seq := mm.seq
	mm.seq++
	to, err := address.NewActorAddress([]byte("destination"))
	require.NoError(mm.t, err)
	msg := NewMessage(
		from,
		to,
		nonce,
		ZeroAttoFIL,
		"method"+fmt.Sprintf("%d", seq),
		[]byte("params"))
	signed, err := NewSignedMessage(*msg, mm.signer, mm.DefaultGasPrice, mm.DefaultGasUnits)
	require.NoError(mm.t, err)
	return signed
}

// EmptyReceipts returns a slice of n empty receipts.
func EmptyReceipts(n int) []*MessageReceipt {
	out := make([]*MessageReceipt, n)
	for i := 0; i < n; i++ {
		out[i] = &MessageReceipt{}
	}
	return out
}
