package types

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
)

// SignedMessage contains a message and its signature
type SignedMessage struct {
	Message   UnsignedMessage  `json:"message"`
	Signature crypto.Signature `json:"signature"`
}

// NewSignedMessage accepts a message `msg` and a signer `s`. NewSignedMessage returns a `SignedMessage` containing
// a signature derived from the serialized `msg` and `msg.From`
// NOTE: this method can only sign message with From being a public-key type address, not an ID address.
// We should deprecate this and move to more explicit signing via an address resolver.
func NewSignedMessage(ctx context.Context, msg UnsignedMessage, s Signer) (*SignedMessage, error) {
	msgCid := msg.Cid()

	sig, err := s.SignBytes(ctx, msgCid.Bytes(), msg.From)
	if err != nil {
		return nil, err
	}

	return &SignedMessage{
		Message:   msg,
		Signature: *sig,
	}, nil
}

// Cid returns the canonical CID for the SignedMessage.
func (smsg *SignedMessage) Cid() cid.Cid {
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.Cid()
	}

	obj, err := smsg.ToNode()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal to marshal message:%s", err))
	}

	return obj.Cid()
}

// ToNode converts the SignedMessage to an IPLD node.
func (smsg *SignedMessage) ToNode() (ipld.Node, error) {
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.ToNode()
	}

	buf := new(bytes.Buffer)
	err := smsg.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
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

// String return message json string
func (smsg *SignedMessage) String() string {
	errStr := "(error encoding SignedMessage)"
	cid := smsg.Cid()
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

//ChainLength return length of message binary
func (smsg *SignedMessage) ChainLength() int {
	var err error
	buf := new(bytes.Buffer)

	if smsg.Signature.Type == crypto.SigTypeBLS {
		// BLS chain message length doesn't include signature
		err = smsg.Message.MarshalCBOR(buf)
	} else {
		err = smsg.MarshalCBOR(buf)
	}
	if err != nil {
		panic(err)
	}
	return buf.Len()
}

// ToStorageBlock return db block of message binary
func (smsg *SignedMessage) ToStorageBlock() (blocks.Block, error) {
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.ToStorageBlock()
	}

	buf := new(bytes.Buffer)
	err := smsg.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

// Serialize return message binary
func (smsg *SignedMessage) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := smsg.MarshalCBOR(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (smsg *SignedMessage) VMMessage() *UnsignedMessage {
	return &smsg.Message
}

var _ ChainMsg = &SignedMessage{}
