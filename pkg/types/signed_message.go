package types

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/xerrors"
	"io"

	"github.com/filecoin-project/go-state-types/abi"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/encoding"
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

func (t *UnsignedMessage) UnmarshalCBOR(r io.Reader) error {
	*t = UnsignedMessage{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 10 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Version (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Version = int64(extra)

	}
	// t.To (address.Address) (struct)

	{

		if err := t.To.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.To: %w", err)
		}

	}
	// t.From (address.Address) (struct)

	{

		if err := t.From.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.From: %w", err)
		}

	}
	// t.Nonce (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Nonce = extra

	}
	// t.Value (big.Int) (struct)

	{

		if err := t.Value.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Value: %w", err)
		}

	}
	// t.GasLimit (int64) (int64)
	{
		maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
		var extraI int64
		if err != nil {
			return err
		}
		switch maj {
		case cbg.MajUnsignedInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 positive overflow")
			}
		case cbg.MajNegativeInt:
			extraI = int64(extra)
			if extraI < 0 {
				return fmt.Errorf("int64 negative oveflow")
			}
			extraI = -1 - extraI
		default:
			return fmt.Errorf("wrong type for int64 field: %d", maj)
		}

		t.GasLimit = Unit(extraI)
	}
	// t.GasFeeCap (big.Int) (struct)

	{

		if err := t.GasFeeCap.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.GasFeeCap: %w", err)
		}

	}
	// t.GasPremium (big.Int) (struct)

	{

		if err := t.GasPremium.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.GasPremium: %w", err)
		}

	}
	// t.Method (abi.MethodNum) (uint64)

	{

		maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Method = abi.MethodNum(extra)

	}
	// t.Params ([]uint8) (slice)

	maj, extra, err = cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if extra > cbg.ByteArrayMaxLen {
		return fmt.Errorf("t.Params: byte array too large (%d)", extra)
	}
	if maj != cbg.MajByteString {
		return fmt.Errorf("expected byte array")
	}

	if extra > 0 {
		t.Params = make([]uint8, extra)
	}

	if _, err := io.ReadFull(br, t.Params[:]); err != nil {
		return err
	}
	return nil
}

func (smsg *SignedMessage) UnmarshalCBOR(r io.Reader) error {
	*smsg = SignedMessage{}

	br := cbg.GetPeeker(r)
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 2 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Message (types.Message) (struct)

	{

		if err := smsg.Message.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Message: %w", err)
		}

	}
	// t.Signature (crypto.Signature) (struct)

	{

		if err := smsg.Signature.UnmarshalCBOR(br); err != nil {
			return xerrors.Errorf("unmarshaling t.Signature: %w", err)
		}

	}
	return nil
}

var _ ChainMsg = &SignedMessage{}
