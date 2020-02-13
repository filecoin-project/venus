package types

import (
	"bytes"
	"encoding/json"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
)

// SignedMessage contains a message and its signature
// TODO do not export these fields as it increases the chances of producing a
// `SignedMessage` with an empty signature.
type SignedMessage struct {
	// control field for encoding struct as an array
	_ struct{} `cbor:",toarray"`

	Message   UnsignedMessage  `json:"meteredMessage"`
	Signature crypto.Signature `json:"signature"`
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
func (smsg *SignedMessage) Cid() (cid.Cid, error) {
	obj, err := smsg.ToNode()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// ToNode converts the SignedMessage to an IPLD node.
func (smsg *SignedMessage) ToNode() (ipld.Node, error) {
	// Use 32 byte / 256 bit digest.
	mhType := uint64(mh.BLAKE2B_MIN + 31)
	mhLen := -1

	data, err := encoding.Encode(smsg)
	if err != nil {
		return nil, err
	}

	hash, err := mh.Sum(data, mhType, mhLen)
	if err != nil {
		return nil, err
	}
	c := cid.NewCidV1(cid.DagCBOR, hash)

	blk, err := blocks.NewBlockWithCid(data, c)
	if err != nil {
		return nil, err
	}
	obj, err := cbor.DecodeBlock(blk)
	if err != nil {
		return nil, err
	}

	return obj, nil

}

// VerifySignature returns true iff the signature is valid for the message content and from address.
func (smsg *SignedMessage) VerifySignature() bool {
	bmsg, err := smsg.Message.Marshal()
	if err != nil {
		return false
	}
	return crypto.IsValidSignature(bmsg, smsg.Message.From, smsg.Signature)
}

// OnChainLen returns the amount of bytes used to represent the message on chain.
func (smsg *SignedMessage) OnChainLen() uint32 {
	panic("byteme")
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
		smsg.Signature.Type == other.Signature.Type &&
		bytes.Equal(smsg.Signature.Data, other.Signature.Data)
}
