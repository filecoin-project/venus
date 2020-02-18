package vm

import (
	"fmt"
	"testing"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/stretchr/testify/require"
)

// MessageMaker creates unique, signed messages for use in tests.
type MessageMaker struct {
	DefaultGasPrice types.AttoFIL
	DefaultGasUnits types.GasUnits

	signer *types.MockSigner
	seq    uint
	t      *testing.T
}

// NewMessageMaker creates a new message maker with a set of signing keys.
func NewMessageMaker(t *testing.T, keys []types.KeyInfo) *MessageMaker {
	addresses := make([]address.Address, len(keys))
	signer := types.NewMockSigner(keys)

	for i, key := range keys {
		addr, _ := key.Address()
		addresses[i] = addr
	}

	return &MessageMaker{types.ZeroAttoFIL, types.NewGasUnits(0), &signer, 0, t}
}

// Addresses returns the addresses for which this maker can sign messages.
func (mm *MessageMaker) Addresses() []address.Address {
	return mm.signer.Addresses
}

// Signer returns the signer with which this maker signs messages.
func (mm *MessageMaker) Signer() *types.MockSigner {
	return mm.signer
}

// NewUnsignedMessage creates a new message.
func (mm *MessageMaker) NewUnsignedMessage(from address.Address, nonce uint64) *types.UnsignedMessage {
	seq := mm.seq
	mm.seq++
	to, err := address.NewSecp256k1Address([]byte("destination"))
	require.NoError(mm.t, err)
	return types.NewMeteredMessage(
		from,
		to,
		nonce,
		types.ZeroAttoFIL,
		types.MethodID(9000+seq),
		[]byte("params"),
		mm.DefaultGasPrice,
		mm.DefaultGasUnits)
}

// NewSignedMessage creates a new signed message.
func (mm *MessageMaker) NewSignedMessage(from address.Address, nonce uint64) *types.SignedMessage {
	msg := mm.NewUnsignedMessage(from, nonce)
	signed, err := types.NewSignedMessage(*msg, mm.signer)
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

// ReceiptMaker generates unique receipts
type ReceiptMaker struct {
	seq uint
}

// NewReceiptMaker creates a new receipt maker
func NewReceiptMaker() *ReceiptMaker {
	return &ReceiptMaker{0}
}

// NewReceipt creates a new distinct receipt.
func (rm *ReceiptMaker) NewReceipt() MessageReceipt {
	seq := rm.seq
	rm.seq++
	return MessageReceipt{
		ReturnValue: []byte(fmt.Sprintf("%d", seq)),
	}
}
