package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-amt-ipld"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	errPkg "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/whyrusleeping/cbor-gen"
)

// GasUnits represents number of units of gas consumed
type GasUnits = Uint64

// BlockGasLimit is the maximum amount of gas that can be used to execute messages in a single block
var BlockGasLimit = NewGasUnits(10000000)

// EmptyMessagesCID is the cid of an empty collection of messages.
var EmptyMessagesCID cid.Cid

// EmptyReceiptsCID is the cid of an empty collection of receipts.
var EmptyReceiptsCID cid.Cid

func init() {
	emptyAMTCid, err := amt.FromArray(amt.WrapBlockstore(blockstore.NewBlockstore(datastore.NewMapDatastore())), []typegen.CBORMarshaler{})
	if err != nil {
		panic("could not create CID for empty AMT")
	}
	EmptyMessagesCID = emptyAMTCid
	EmptyReceiptsCID = emptyAMTCid
}

var (
	// ErrInvalidMessageLength is returned when the message length does not match the expected length.
	ErrInvalidMessageLength = errors.New("invalid message length")
)

// UnsignedMessage is an exchange of information between two actors modeled
// as a function call.
// Messages are the equivalent of transactions in Ethereum.
type UnsignedMessage struct {
	To   address.Address `json:"to"`
	From address.Address `json:"from"`
	// When receiving a message from a user account the nonce in
	// the message must match the expected nonce in the from actor.
	// This prevents replay attacks.
	CallSeqNum Uint64 `json:"callSeqNum"`

	Value AttoFIL `json:"value"`

	Method string `json:"method"`
	Params []byte `json:"params"`

	GasPrice AttoFIL  `json:"gasPrice"`
	GasLimit GasUnits `json:"gasLimit"`
	// Pay attention to Equals() if updating this struct.
}

// NewUnsignedMessage creates a new message.
func NewUnsignedMessage(from, to address.Address, nonce uint64, value AttoFIL, method string, params []byte) *UnsignedMessage {
	return &UnsignedMessage{
		From:       from,
		To:         to,
		CallSeqNum: Uint64(nonce),
		Value:      value,
		Method:     method,
		Params:     params,
	}
}

// NewMeteredMessage adds gas price and gas limit to the message
func NewMeteredMessage(from, to address.Address, nonce uint64, value AttoFIL, method string, params []byte, price AttoFIL, limit GasUnits) *UnsignedMessage {
	return &UnsignedMessage{
		From:       from,
		To:         to,
		CallSeqNum: Uint64(nonce),
		Value:      value,
		Method:     method,
		Params:     params,
		GasPrice:   price,
		GasLimit:   limit,
	}
}

// Unmarshal a message from the given bytes.
func (msg *UnsignedMessage) Unmarshal(b []byte) error {
	return encoding.Decode(b, msg)
}

// Marshal the message into bytes.
func (msg *UnsignedMessage) Marshal() ([]byte, error) {
	return encoding.Encode(msg)
}

// ToNode converts the Message to an IPLD node.
func (msg *UnsignedMessage) ToNode() (ipld.Node, error) {
	// Use 32 byte / 256 bit digest.
	obj, err := cbor.WrapObject(msg, DefaultHashFunction, -1)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// Cid returns the canonical CID for the message.
// TODO: can we avoid returning an error?
func (msg *UnsignedMessage) Cid() (cid.Cid, error) {
	obj, err := msg.ToNode()
	if err != nil {
		return cid.Undef, errPkg.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

func (msg *UnsignedMessage) String() string {
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
func (msg *UnsignedMessage) Equals(other *UnsignedMessage) bool {
	return msg.To == other.To &&
		msg.From == other.From &&
		msg.CallSeqNum == other.CallSeqNum &&
		msg.Value.Equal(other.Value) &&
		msg.Method == other.Method &&
		msg.GasPrice.Equal(other.GasPrice) &&
		msg.GasLimit == other.GasLimit &&
		bytes.Equal(msg.Params, other.Params)
}

// SignedMessageCollection tracks a group of messages and assigns it a cid.
type SignedMessageCollection []*SignedMessage

// DecodeSignedMessages decodes raw bytes into an array of signed messages
func DecodeSignedMessages(b []byte) ([]*SignedMessage, error) {
	var out SignedMessageCollection
	if err := encoding.Decode(b, &out); err != nil {
		return nil, err
	}

	return []*SignedMessage(out), nil
}

// TODO #3078 the panics here and in types.Block should go away.  We need to
// keep them in order to use the ipld cborstore with the default hash function
// because we need to implement hamt.cidProvider which doesn't handle errors.
// We can clean all this up when we can use our own CborIpldStore with the hamt.

// Cid returns the cid of the message collection.
func (mC SignedMessageCollection) Cid() cid.Cid {
	return mC.ToNode().Cid()
}

// ToNode converts the collection to an IPLD node.
func (mC SignedMessageCollection) ToNode() ipld.Node {
	obj, err := cbor.WrapObject(mC, DefaultHashFunction, -1)
	if err != nil {
		panic(err)
	}

	return obj
}

// MessageCollection tracks a group of messages and assigns it a cid.
type MessageCollection []*UnsignedMessage

// DecodeMessages decodes raw bytes into an array of metered messages
func DecodeMessages(b []byte) ([]*UnsignedMessage, error) {
	var out MessageCollection
	if err := encoding.Decode(b, &out); err != nil {
		return nil, err
	}

	return []*UnsignedMessage(out), nil
}

// Cid returns the cid of the message collection.
func (mC MessageCollection) Cid() cid.Cid {
	return mC.ToNode().Cid()
}

// ToNode converts the collection to an IPLD node.
func (mC MessageCollection) ToNode() ipld.Node {
	obj, err := cbor.WrapObject(mC, DefaultHashFunction, -1)
	if err != nil {
		panic(err)
	}

	return obj
}

// ReceiptCollection tracks a group of receipts and assigns it a cid.
type ReceiptCollection []*MessageReceipt

// DecodeReceipts decodes raw bytes into an array of message receipts
func DecodeReceipts(b []byte) ([]*MessageReceipt, error) {
	var out ReceiptCollection
	if err := encoding.Decode(b, &out); err != nil {
		return nil, err
	}

	return []*MessageReceipt(out), nil
}

// Cid returns the cid of the receipt collection.
func (rC ReceiptCollection) Cid() cid.Cid {
	return rC.ToNode().Cid()
}

// ToNode converts the collection to an IPLD node.
func (rC ReceiptCollection) ToNode() ipld.Node {
	obj, err := cbor.WrapObject(rC, DefaultHashFunction, -1)
	if err != nil {
		panic(err)
	}

	return obj
}

// NewGasPrice constructs a gas price (in AttoFIL) from the given number.
func NewGasPrice(price int64) AttoFIL {
	return NewAttoFIL(big.NewInt(price))
}

// NewGasUnits constructs a new GasUnits from the given number.
func NewGasUnits(cost uint64) GasUnits {
	return Uint64(cost)
}

// TxMeta tracks the merkleroots of both secp and bls messages separately
type TxMeta struct {
	SecpRoot cid.Cid `json:"secpRoot"`
	BLSRoot  cid.Cid `json:"blsRoot"`
}

// String returns a readable printing string of TxMeta
func (m TxMeta) String() string {
	return fmt.Sprintf("secp: %s, bls: %s", m.SecpRoot.String(), m.BLSRoot.String())
}
