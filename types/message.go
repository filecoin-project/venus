package types

import (
	"errors"
	"math/big"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	errPkg "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(Message{})
}

var (
	// ErrInvalidMessageLength is returned when the message length does not match the expected length.
	ErrInvalidMessageLength = errors.New("invalid message length")
)

// Message is an exchange of information between two actors modeled
// as a function call.
// Messages are the equivalent of transactions in Ethereum.
type Message struct {
	To   Address
	From Address

	Value *big.Int

	Method string
	Params []byte
}

// Unmarshal a message from the given bytes.
func (msg *Message) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, msg)
}

// Marshal the message into bytes.
func (msg *Message) Marshal() ([]byte, error) {
	return cbor.DumpObject(msg)
}

// Cid returns the canonical CID for the message.
// TODO: can we avoid returning an error?
func (msg *Message) Cid() (*cid.Cid, error) {
	obj, err := cbor.WrapObject(msg, DefaultHashFunction, -1)
	if err != nil {
		return nil, errPkg.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// NewMessage creates a new message.
func NewMessage(from, to Address, value *big.Int, method string, params []byte) *Message {
	return &Message{
		From:   from,
		To:     to,
		Value:  value,
		Method: method,
		Params: params,
	}
}
