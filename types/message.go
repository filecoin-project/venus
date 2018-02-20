package types

import (
	"errors"
	"fmt"

	atlas "gx/ipfs/QmSaDQWMxJBMtzQWnGoDppbwSEbHv4aJcD86CMSdszPU4L/refmt/obj/atlas"
	errPkg "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(messageCborEntry)
}

var (
	// ErrInvalidMessageLength is returned when the message length does not match the expected length.
	ErrInvalidMessageLength = errors.New("invalid message length")
)

// Message is an exchange of information between two actors modeled
// as a function call.
// Messages are the equivalent of transactions in Ethereum.
type Message struct {
	to   Address
	from Address

	method string
	params []interface{}
}

// To is a getter for the to field.
func (msg *Message) To() Address { return msg.to }

// From is a getter for the from field.
func (msg *Message) From() Address { return msg.from }

// Method is a getter for the method field.
func (msg *Message) Method() string { return msg.method }

// Params is a getter for the params field.
func (msg *Message) Params() []interface{} { return msg.params }

// HasFrom indidcates if this message has a from address.
func (msg *Message) HasFrom() bool {
	return msg.from != Address("")
}

var messageCborEntry = atlas.
	BuildEntry(Message{}).
	Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(marshalMessage)).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(unmarshalMessage)).
	Complete()

func marshalMessage(msg Message) ([]interface{}, error) {
	return []interface{}{
		msg.to,
		msg.from,
		msg.method,
		msg.params,
	}, nil
}

func unmarshalMessage(x []interface{}) (Message, error) {
	if len(x) != 4 {
		return Message{}, ErrInvalidMessageLength
	}

	to, ok := x[0].(string)
	if !ok {
		return Message{}, errInvalidMessage("to", x[0])
	}

	from, ok := x[1].(string)
	if !ok {
		return Message{}, errInvalidMessage("from", x[1])
	}

	method, ok := x[2].(string)
	if !ok {
		return Message{}, errInvalidMessage("method", x[2])
	}

	params, ok := x[3].([]interface{})
	if !ok && x[3] != nil {
		return Message{}, errInvalidMessage("params", x[3])
	}

	return Message{
		to:     Address(to),
		from:   Address(from),
		method: method,
		params: params,
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

// NewMessage creates a new message.
func NewMessage(from, to Address, method string, params []interface{}) *Message {
	return &Message{
		from:   from,
		to:     to,
		method: method,
		params: params,
	}
}

func errInvalidMessage(field string, received interface{}) error {
	return fmt.Errorf("invalid message %s field: %v", field, received)
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
			Address(s+"-from"),
			Address(s+"-to"),
			s+"-method",
			nil)
	}
}
