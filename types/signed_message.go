package types

import (
	"encoding/json"
	"fmt"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
)

var (
	// ErrMessageSigned is returned when `Sign()` is called on a signedmessage that has previously been signed
	ErrMessageSigned = errors.New("message already contains a signature")
	// ErrMessageUnsigned is returned when `RecoverAddress` is called on a signedmessage that does not contain a signature
	ErrMessageUnsigned = errors.New("message does not contain a signature")
)

func init() {
	cbor.RegisterCborType(SignedMessage{})
}

// SignedMessage contains a message and its signature
// TODO do not export these fields as it increases the chances of producing a
// `SignedMessage` with an empty signature.
type SignedMessage struct {
	MeteredMessage `json:"meteredMessage"`
	Signature      Signature `json:"signature"`
}

// Unmarshal a SignedMessage from the given bytes.
func (smsg *SignedMessage) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, smsg)
}

// Marshal the SignedMessage into bytes.
func (smsg *SignedMessage) Marshal() ([]byte, error) {
	return cbor.DumpObject(smsg)
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

// RecoverAddress returns the address derived from the signature and message encapsulated in `SignedMessage`
func (smsg *SignedMessage) RecoverAddress(r Recoverer) (address.Address, error) {
	if len(smsg.Signature) < 1 {
		return address.Address{}, ErrMessageUnsigned
	}

	bmsg, err := smsg.MeteredMessage.Marshal()
	if err != nil {
		return address.Address{}, err
	}

	maybePk, err := r.Ecrecover(bmsg, smsg.Signature)
	if err != nil {
		return address.Address{}, err
	}

	maybeAddrHash := address.Hash(maybePk)

	return address.NewMainnet(maybeAddrHash), nil

}

// VerifySignature returns true iff the signature over the message as calculated
// from EC recover matches the message sender address.
func (smsg *SignedMessage) VerifySignature() bool {
	bmsg, err := smsg.MeteredMessage.Marshal()
	if err != nil {
		log.Infof("invalid signature: %s", err)
		return false
	}
	return IsValidSignature(bmsg, smsg.From, smsg.Signature)
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

// NewSignedMessage accepts a message `msg` and a signer `s`. NewSignedMessage returns a `SignedMessage` containing
// a signature derived from the serialized `msg` and `msg.From`
func NewSignedMessage(msg Message, s Signer, gasPrice AttoFIL, gasLimit GasUnits) (*SignedMessage, error) {
	meteredMsg := NewMeteredMessage(msg, gasPrice, gasLimit)

	bmsg, err := meteredMsg.Marshal()
	if err != nil {
		return nil, err
	}

	sig, err := s.SignBytes(bmsg, msg.From)
	if err != nil {
		return nil, err
	}

	return &SignedMessage{
		MeteredMessage: *meteredMsg,
		Signature:      sig,
	}, nil
}
