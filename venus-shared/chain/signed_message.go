package chain

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

// SignedMessage contains a message and its signature
type SignedMessage struct {
	Message   Message
	Signature crypto.Signature
}

func (smsg *SignedMessage) ChainLength() int {
	var data []byte
	var err error
	if smsg.Signature.Type == crypto.SigTypeBLS {
		data, err = smsg.Message.Serialize()
	} else {
		data, err = smsg.Serialize()
	}

	if err != nil {
		panic(err)
	}

	return len(data)
}

// Serialize return message binary
func (smsg *SignedMessage) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := smsg.MarshalCBOR(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Serialize return message binary
func (smsg *SignedMessage) SerializeWithCid() (cid.Cid, []byte, error) {
	data, err := smsg.Serialize()
	if err != nil {
		return cid.Undef, nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return cid.Undef, nil, err
	}

	return c, data, nil
}

func (smsg *SignedMessage) ToStorageBlock() (blocks.Block, error) {
	var c cid.Cid
	var data []byte
	var err error
	if smsg.Signature.Type == crypto.SigTypeBLS {
		c, data, err = smsg.Message.SerializeWithCid()
	} else {
		c, data, err = smsg.SerializeWithCid()
	}

	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (smsg *SignedMessage) Cid() cid.Cid {
	if smsg.Signature.Type == crypto.SigTypeBLS {
		return smsg.Message.Cid()
	}

	c, _, err := smsg.SerializeWithCid()
	if err != nil {
		panic(fmt.Errorf("failed to marshal signed-message: %w", err))
	}

	return c
}

// String return message json string
func (smsg *SignedMessage) String() string {
	errStr := "(error encoding SignedMessage)"
	c, _, err := smsg.SerializeWithCid()
	if err != nil {
		return errStr
	}

	js, err := json.MarshalIndent(smsg, "", "  ")
	if err != nil {
		return errStr
	}

	return fmt.Sprintf("SignedMessage cid=[%v]: %s", c, string(js))
}
