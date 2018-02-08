package types

import (
	"errors"
	"math/big"

	atlas "gx/ipfs/QmSaDQWMxJBMtzQWnGoDppbwSEbHv4aJcD86CMSdszPU4L/refmt/obj/atlas"
	errPkg "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(messageCborEntry)
}

var (
	ErrInvalidMessageLength      = errors.New("invalid message length")
	ErrInvalidMessageToField     = errors.New("invalid message to field")
	ErrInvalidMessageFromField   = errors.New("invalid message from field")
	ErrInvalidMessageValueField  = errors.New("invalid message value field")
	ErrInvalidMessageMethodField = errors.New("invalid message method field")
	ErrInvalidMessageParamsField = errors.New("invalid message params field")
)

// Message is the equivalent of an Ethereum transaction. They are the way actors exchange information between each other.
type Message struct {
	To   Address
	From Address

	Value *big.Int

	Method string
	Params []interface{}
}

var messageCborEntry = atlas.
	BuildEntry(Message{}).
	Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(marshalMessage)).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(unmarshalMessage)).
	Complete()

func marshalMessage(msg Message) ([]interface{}, error) {
	return []interface{}{
		msg.To,
		msg.From,
		msg.Value,
		msg.Method,
		msg.Params,
	}, nil
}

func unmarshalMessage(x []interface{}) (Message, error) {
	if len(x) != 5 {
		return Message{}, ErrInvalidMessageLength
	}

	to, ok := x[0].(string)
	if !ok {
		return Message{}, ErrInvalidMessageToField
	}

	from, ok := x[1].(string)
	if !ok {
		return Message{}, ErrInvalidMessageFromField
	}

	valueB, ok := x[2].([]byte)
	if !ok && x[2] != nil {
		return Message{}, ErrInvalidMessageValueField
	}

	method, ok := x[3].(string)
	if !ok {
		return Message{}, ErrInvalidMessageMethodField
	}

	params, ok := x[4].([]interface{})
	if !ok && x[4] != nil {
		return Message{}, ErrInvalidMessageParamsField
	}

	return Message{
		To:     Address(to),
		From:   Address(from),
		Value:  big.NewInt(0).SetBytes(valueB),
		Method: method,
		Params: params,
	}, nil
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
