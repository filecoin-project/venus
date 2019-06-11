package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	errPkg "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
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
	To   address.Address `json:"to"`
	From address.Address `json:"from"`
	// When receiving a message from a user account the nonce in
	// the message must match the expected nonce in the from actor.
	// This prevents replay attacks.
	Nonce Uint64 `json:"nonce"`

	Value AttoFIL `json:"value"`

	Method string `json:"method"`
	Params []byte `json:"params"`
	// Pay attention to Equals() if updating this struct.
}

// NewMessage creates a new message.
func NewMessage(from, to address.Address, nonce uint64, value AttoFIL, method string, params []byte) *Message {
	return &Message{
		From:   from,
		To:     to,
		Nonce:  Uint64(nonce),
		Value:  value,
		Method: method,
		Params: params,
	}
}

// Unmarshal a message from the given bytes.
func (msg *Message) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, msg)
}

// Marshal the message into bytes.
func (msg *Message) Marshal() ([]byte, error) {
	return cbor.DumpObject(msg)
}

// ToNode converts the Message to an IPLD node.
func (msg *Message) ToNode() (ipld.Node, error) {
	// Use 32 byte / 256 bit digest.
	obj, err := cbor.WrapObject(msg, DefaultHashFunction, -1)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// Cid returns the canonical CID for the message.
// TODO: can we avoid returning an error?
func (msg *Message) Cid() (cid.Cid, error) {
	obj, err := msg.ToNode()
	if err != nil {
		return cid.Undef, errPkg.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

func (msg *Message) String() string {
	errStr := "(error encoding Message)"
	cid, err := msg.Cid()
	if err != nil {
		return errStr
	}
	js, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("Message cid=[%v]: %s", cid, string(js))
}

// Equals tests whether two messages are equal
func (msg *Message) Equals(other *Message) bool {
	return msg.To == other.To &&
		msg.From == other.From &&
		msg.Nonce == other.Nonce &&
		msg.Value.Equal(other.Value) &&
		msg.Method == other.Method &&
		bytes.Equal(msg.Params, other.Params)
}
