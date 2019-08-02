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

// EmptyMessagesCID is the cid of an empty collection of messages.
var EmptyMessagesCID = MessageCollection{}.Cid()

// EmptyReceiptsCID is the cid of an empty collection of receipts.
var EmptyReceiptsCID = ReceiptCollection{}.Cid()

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

// MessageCollection tracks a group of messages and assigns it a cid.
type MessageCollection []*SignedMessage

// TODO #3078 the panics here and in types.Block should go away.  We need to
// keep them in order to use the ipld cborstore with the default hash function
// because we need to implement hamt.cidProvider which doesn't handle errors.
// We can clean all this up when we can use our own CborIpldStore with the hamt.

// Cid returns the cid of the message collection.
func (mC MessageCollection) Cid() cid.Cid {
	nd, err := cbor.WrapObject(mC, DefaultHashFunction, -1)
	if err != nil {
		panic(err)
	}

	return nd.Cid()
}

// ReceiptCollection tracks a group of receipts and assigns it a cid.
type ReceiptCollection []*MessageReceipt

// Cid returns the cid of the receipt collection.
func (rC ReceiptCollection) Cid() cid.Cid {
	nd, err := cbor.WrapObject(rC, DefaultHashFunction, -1)
	if err != nil {
		panic(err)
	}

	return nd.Cid()
}
