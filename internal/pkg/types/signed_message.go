package types

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/pkg/errors"
)

var (
	// ErrMessageSigned is returned when `Sign()` is called on a signedmessage that has previously been signed
	ErrMessageSigned = errors.New("message already contains a signature")
	// ErrMessageUnsigned is returned when `RecoverAddress` is called on a signedmessage that does not contain a signature
	ErrMessageUnsigned = errors.New("message does not contain a signature")
)

// SignedMessage contains a message and its signature
// TODO do not export these fields as it increases the chances of producing a
// `SignedMessage` with an empty signature.
type SignedMessage struct {
	Message   UnsignedMessage `json:"meteredMessage"`
	Signature Signature       `json:"signature"`
	// Pay attention to Equals() if updating this struct.
}

// NewSignedMessage accepts a message `msg` and a signer `s`. NewSignedMessage returns a `SignedMessage` containing
// a signature derived from the serialized `msg` and `msg.From`
func NewSignedMessage(msg UnsignedMessage, s Signer) (*SignedMessage, error) {
	msgData, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	sig, err := s.SignBytes(msgData, msg.From)
	if err != nil {
		return nil, err
	}

	return &SignedMessage{
		Message:   msg,
		Signature: sig,
	}, nil
}

// UnwrapSigned returns the unsigned messages from a slice of signed messages.
func UnwrapSigned(smsgs []*SignedMessage) []*UnsignedMessage {
	unsigned := make([]*UnsignedMessage, len(smsgs))
	for i, sm := range smsgs {
		unsigned[i] = &sm.Message
	}
	return unsigned
}

// Unmarshal a SignedMessage from the given bytes.
func (smsg *SignedMessage) Unmarshal(b []byte) error {
	return encoding.Decode(b, smsg)
}

// Marshal the SignedMessage into bytes.
func (smsg *SignedMessage) Marshal() ([]byte, error) {
	return encoding.Encode(smsg)
}

// Cid returns the canonical CID for the SignedMessage.
// TODO: can we avoid returning an error?
func (smsg *SignedMessage) Cid() (cid.Cid, error) {
	obj, err := cbor.WrapObject(smsg, DefaultHashFunction, -1)
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// ToNode converts the SignedMessage to an IPLD node.
func (smsg *SignedMessage) ToNode() (ipld.Node, error) {
	// Use 32 byte / 256 bit digest.
	obj, err := cbor.WrapObject(smsg, DefaultHashFunction, -1)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// VerifySignature returns true iff the signature is valid for the message content and from address.
func (smsg *SignedMessage) VerifySignature() bool {
	bmsg, err := smsg.Message.Marshal()
	if err != nil {
		log.Infof("invalid signature: %s", err)
		return false
	}
	return IsValidSignature(bmsg, smsg.Message.From, smsg.Signature)
}

func (smsg *SignedMessage) String() string {
	errStr := "(error encoding SignedMessage)"
	cid, err := smsg.Cid()
	if err != nil {
		return errStr
	}
	js, err := json.MarshalIndent(smsg, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("SignedMessage cid=[%v]: %s", cid, string(js))
}

// Equals tests whether two signed messages are equal.
func (smsg *SignedMessage) Equals(other *SignedMessage) bool {
	return smsg.Message.Equals(&other.Message) &&
		bytes.Equal(smsg.Signature, other.Signature)
}
