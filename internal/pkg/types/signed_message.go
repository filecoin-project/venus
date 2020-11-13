package types

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
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
// NOTE: this method can only sign message with From being a public-key type address, not an ID address.
// We should deprecate this and move to more explicit signing via an address resolver.
func NewSignedMessage(ctx context.Context, msg UnsignedMessage, s Signer) (*SignedMessage, error) {
	msgCid, err := msg.Cid()
	if err != nil {
		return nil, err
	}

	sig, err := s.SignBytes(ctx, msgCid.Bytes(), msg.From)
	if err != nil {
		return nil, err
	}

	return &SignedMessage{
		Message:   msg,
		Signature: sig,
	}, nil
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
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.Cid()
	}

	obj, err := smsg.ToNode()
	if err != nil {
		return cid.Undef, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// ToNode converts the SignedMessage to an IPLD node.
func (smsg *SignedMessage) ToNode() (ipld.Node, error) {
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.ToNode()
	}

	data, err := encoding.Encode(smsg)
	if err != nil {
		return nil, err
	}
	c, err := constants.DefaultCidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

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
		smsg.Signature.Equals(&other.Signature)
}

func (smsg *SignedMessage) ChainLength() int {
	ser, err := smsg.Marshal()
	if err != nil {
		panic(err)
	}
	return len(ser)
}

func (smsg *SignedMessage) ToStorageBlock() (blocks.Block, error) {
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.ToStorageBlock()
	}

	data, err := smsg.Marshal()
	if err != nil {
		return nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (smsg *SignedMessage) VMMessage() *UnsignedMessage {
	return &smsg.Message
}

var _ ChainMsg = &SignedMessage{}
