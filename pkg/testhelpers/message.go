package testhelpers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/crypto"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

// NewMessage creates a new message.
func NewMessage(from, to address.Address, nonce uint64, value abi.TokenAmount, method abi.MethodNum, params []byte) *types.Message {
	return &types.Message{
		Version: 0,
		To:      to,
		From:    from,
		Nonce:   nonce,
		Value:   value,
		Method:  method,
		Params:  params,
	}
}

// NewMeteredMessage adds gas price and gas limit to the message
func NewMeteredMessage(from, to address.Address, nonce uint64, value abi.TokenAmount, method abi.MethodNum, params []byte, gasFeeCap, gasPremium abi.TokenAmount, limit int64) *types.Message {
	return &types.Message{
		Version:    0,
		To:         to,
		From:       from,
		Nonce:      nonce,
		Value:      value,
		GasFeeCap:  gasFeeCap,
		GasPremium: gasPremium,
		GasLimit:   limit,
		Method:     method,
		Params:     params,
	}
}

// NewSignedMessage accepts a message `msg` and a signer `s`. NewSignedMessage returns a `SignedMessage` containing
// a signature derived from the serialized `msg` and `msg.From`
// NOTE: this method can only sign message with From being a public-key type address, not an ID address.
// We should deprecate this and move to more explicit signing via an address resolver.
func NewSignedMessage(ctx context.Context, msg types.Message, s types.Signer) (*types.SignedMessage, error) {
	msgCid := msg.Cid()

	sig, err := s.SignBytes(ctx, msgCid.Bytes(), msg.From)
	if err != nil {
		return nil, err
	}

	return &types.SignedMessage{
		Message:   msg,
		Signature: *sig,
	}, nil
}

// NewSignedMessageForTestGetter returns a closure that returns a SignedMessage unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
// TODO support chosing from address
func NewSignedMessageForTestGetter(ms MockSigner) func(uint64) *types.SignedMessage {
	i := 0
	return func(nonce uint64) *types.SignedMessage {
		s := fmt.Sprintf("smsg%d", i)
		i++
		newAddr, err := address.NewSecp256k1Address([]byte(s + "-to"))
		if err != nil {
			panic(err)
		}
		msg := NewMeteredMessage(
			ms.Addresses[0], // from needs to be an address from the signer
			newAddr,
			nonce,
			types.ZeroFIL,
			0,
			[]byte("params"),
			types.ZeroFIL,
			types.ZeroFIL,
			0,
		)
		smsg, err := NewSignedMessage(context.TODO(), *msg, &ms)
		if err != nil {
			panic(err)
		}
		return smsg
	}
}

// NewCidForTestGetter returns a closure that returns a Cid unique to that invocation.
// The Cid is unique wrt the closure returned, not globally. You can use this function
// in tests.
func NewCidForTestGetter() func() cid.Cid {
	i := 31337
	return func() cid.Cid {
		obj, err := cbor.WrapObject([]int{i}, constants.DefaultHashFunction, -1)
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
func NewMessageForTestGetter() func() *types.Message {
	i := 0
	return func() *types.Message {
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
		return NewMessage(
			from,
			to,
			0,
			types.ZeroFIL,
			abi.MethodNum(10000+i),
			nil)
	}
}

// NewMsgs returns n messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewMsgs will return the same
// set of messages).
func NewMsgs(n int) []*types.Message {
	newMsg := NewMessageForTestGetter()
	msgs := make([]*types.Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
		msgs[i].Nonce = uint64(i)
	}
	return msgs
}

// NewSignedMsgs returns n signed messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewSignedMsgs will return the same
// set of messages).
func NewSignedMsgs(n uint, ms MockSigner) []*types.SignedMessage {
	var err error
	newMsg := NewMessageForTestGetter()
	smsgs := make([]*types.SignedMessage, n)
	for i := uint(0); i < n; i++ {
		msg := newMsg()
		msg.From = ms.Addresses[0]
		msg.Nonce = uint64(i)
		msg.GasFeeCap = types.ZeroFIL
		msg.GasPremium = types.ZeroFIL
		msg.GasLimit = 0
		smsgs[i], err = NewSignedMessage(context.TODO(), *msg, ms)
		if err != nil {
			panic(err)
		}
	}
	return smsgs
}

// SignMsgs returns a slice of signed messages where the original messages
// are `msgs`, if signing one of the `msgs` fails an error is returned
func SignMsgs(ms MockSigner, msgs []*types.Message) ([]*types.SignedMessage, error) {
	var smsgs []*types.SignedMessage
	for _, m := range msgs {
		s, err := NewSignedMessage(context.TODO(), *m, ms)
		if err != nil {
			return nil, err
		}
		smsgs = append(smsgs, s)
	}
	return smsgs, nil
}

// NewMsgsWithAddrs returns a slice of `n` messages who's `From` field's are pulled
// from `a`. This method should be used when the addresses returned are to be signed
// at a later point.
func NewMsgsWithAddrs(n int, a []address.Address) []*types.Message {
	if n > len(a) {
		panic("cannot create more messages than there are addresess for")
	}
	newMsg := NewMessageForTestGetter()
	msgs := make([]*types.Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
		msgs[i].From = a[i]
	}
	return msgs
}

// MessageMaker creates unique, signed messages for use in tests.
type MessageMaker struct {
	DefaultGasFeeCap  types.BigInt
	DefaultGasPremium types.BigInt
	DefaultGasUnits   int64

	signer *MockSigner
	seq    uint
	t      *testing.T
}

// NewMessageMaker creates a new message maker with a set of signing keys.
func NewMessageMaker(t *testing.T, keys []crypto.KeyInfo) *MessageMaker {
	addresses := make([]address.Address, len(keys))
	signer := NewMockSigner(keys)

	for i, key := range keys {
		addr, _ := key.Address()
		addresses[i] = addr
	}

	return &MessageMaker{types.ZeroFIL, types.ZeroFIL, 0, &signer, 0, t}
}

// Addresses returns the addresses for which this maker can sign messages.
func (mm *MessageMaker) Addresses() []address.Address {
	return mm.signer.Addresses
}

// Signer returns the signer with which this maker signs messages.
func (mm *MessageMaker) Signer() *MockSigner {
	return mm.signer
}

// NewUnsignedMessage creates a new message.
func (mm *MessageMaker) NewUnsignedMessage(from address.Address, nonce uint64) *types.Message {
	seq := mm.seq
	mm.seq++
	to, err := address.NewSecp256k1Address([]byte("destination"))
	require.NoError(mm.t, err)
	return NewMeteredMessage(
		from,
		to,
		nonce,
		types.ZeroFIL,
		abi.MethodNum(9000+seq),
		[]byte("params"),
		mm.DefaultGasFeeCap,
		mm.DefaultGasPremium,
		mm.DefaultGasUnits)
}

// NewSignedMessage creates a new signed message.
func (mm *MessageMaker) NewSignedMessage(from address.Address, nonce uint64) *types.SignedMessage {
	msg := mm.NewUnsignedMessage(from, nonce)
	signed, err := NewSignedMessage(context.TODO(), *msg, mm.signer)
	require.NoError(mm.t, err)
	return signed
}

// EmptyReceipts returns a slice of n empty receipts.
func EmptyReceipts(n int) []*types.MessageReceipt {
	out := make([]*types.MessageReceipt, n)
	for i := 0; i < n; i++ {
		out[i] = &types.MessageReceipt{}
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
func (rm *ReceiptMaker) NewReceipt() types.MessageReceipt {
	seq := rm.seq
	rm.seq++
	return types.MessageReceipt{
		Return: []byte(fmt.Sprintf("%d", seq)),
	}
}
