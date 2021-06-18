package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	specsbig "github.com/filecoin-project/go-state-types/big"
	cbor2 "github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/constants"
)

type EstimateMessage struct {
	Msg  *Message
	Spec *MessageSendSpec
}

type EstimateResult struct {
	Msg *Message
	Err string
}

type MessageSendSpec struct {
	MaxFee            abi.TokenAmount
	GasOverEstimation float64
}

var DefaultMessageSendSpec = MessageSendSpec{
	// MaxFee of 0.1FIL
	MaxFee: abi.NewTokenAmount(int64(constants.FilecoinPrecision) / 10),
}

func (ms *MessageSendSpec) Get() MessageSendSpec {
	if ms == nil {
		return DefaultMessageSendSpec
	}

	return *ms
}

const MessageVersion = 0

var EmptyTokenAmount = abi.TokenAmount{}

//
type ChainMsg interface {
	Cid() cid.Cid
	VMMessage() *UnsignedMessage
	ToStorageBlock() (block.Block, error)
	// FIXME: This is the *message* length, this name is misleading.
	ChainLength() int
	cbor2.Marshaler
	cbor2.Unmarshaler
}

var _ ChainMsg = &UnsignedMessage{}

type Message = UnsignedMessage

// UnsignedMessage is an exchange of information between two actors modeled
// as a function call.
type UnsignedMessage struct {
	Version uint64 `json:"version"`

	To   address.Address `json:"to"`
	From address.Address `json:"from"`
	// When receiving a message from a user account the nonce in
	// the message must match the expected nonce in the from actor.
	// This prevents replay attacks.
	Nonce uint64 `json:"nonce"`

	Value abi.TokenAmount `json:"value"`

	GasLimit   int64           `json:"gasLimit"`
	GasFeeCap  abi.TokenAmount `json:"gasFeeCap"`
	GasPremium abi.TokenAmount `json:"gasPremium"`

	Method abi.MethodNum `json:"method"`
	Params []byte        `json:"params"`
}

// NewUnsignedMessage creates a new message.
func NewUnsignedMessage(from, to address.Address, nonce uint64, value abi.TokenAmount, method abi.MethodNum, params []byte) *UnsignedMessage {
	return &UnsignedMessage{
		Version: MessageVersion,
		To:      to,
		From:    from,
		Nonce:   nonce,
		Value:   value,
		Method:  method,
		Params:  params,
	}
}

// NewMeteredMessage adds gas price and gas limit to the message
func NewMeteredMessage(from, to address.Address, nonce uint64, value abi.TokenAmount, method abi.MethodNum, params []byte, gasFeeCap, gasPremium abi.TokenAmount, limit int64) *UnsignedMessage {
	return &UnsignedMessage{
		Version:    MessageVersion,
		To:         to,
		From:       from,
		Nonce:      nonce,
		Value:      value,
		GasFeeCap:  gasFeeCap,
		GasPremium: gasPremium,
		GasLimit:   limit,
		Method:     method,
		Params:     params,
	}
}

func (msg *UnsignedMessage) RequiredFunds() abi.TokenAmount {
	return specsbig.Mul(msg.GasFeeCap, specsbig.NewInt(msg.GasLimit))
}

// ToNode converts the Message to an IPLD node.
func (msg *UnsignedMessage) ToNode() (ipld.Node, error) {
	buf := new(bytes.Buffer)
	err := msg.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	c, err := constants.DefaultCidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	blk, err := block.NewBlockWithCid(data, c)
	if err != nil {
		return nil, err
	}
	obj, err := cbor.DecodeBlock(blk)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// Cid returns the canonical CID for the message.
// TODO: can we avoid returning an error?
func (msg *UnsignedMessage) Cid() cid.Cid {
	obj, err := msg.ToNode()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal to marshal unsigned message:%s", err))
	}

	return obj.Cid()
}

func (msg *UnsignedMessage) String() string {
	errStr := "(error encoding Message)"
	cid := msg.Cid()
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
		msg.Nonce == other.Nonce &&
		msg.Value.Equals(other.Value) &&
		msg.GasPremium.Equals(other.GasPremium) &&
		msg.GasFeeCap.Equals(other.GasFeeCap) &&
		msg.GasLimit == other.GasLimit &&
		msg.Method == other.Method &&
		bytes.Equal(msg.Params, other.Params)
}

func (msg *UnsignedMessage) EqualCall(o *Message) bool {
	m1 := *msg
	m2 := *o

	m1.GasLimit, m2.GasLimit = 0, 0
	m1.GasFeeCap, m2.GasFeeCap = specsbig.Zero(), specsbig.Zero()
	m1.GasPremium, m2.GasPremium = specsbig.Zero(), specsbig.Zero()

	return (&m1).Equals(&m2)
}

func (msg *UnsignedMessage) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := msg.MarshalCBOR(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (msg *UnsignedMessage) ChainLength() int {
	buf := new(bytes.Buffer)
	err := msg.MarshalCBOR(buf)
	if err != nil {
		panic(err)
	}

	return buf.Len()
}

func (msg *UnsignedMessage) VMMessage() *UnsignedMessage {
	return msg
}

func (msg *UnsignedMessage) ToStorageBlock() (block.Block, error) {
	buf := new(bytes.Buffer)
	err := msg.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	data := buf.Bytes()
	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	return block.NewBlockWithCid(data, c)
}

func (msg *UnsignedMessage) ValidForBlockInclusion(minGas int64, version network.Version) error {
	if msg.Version != 0 {
		return xerrors.New("'Version' unsupported")
	}

	if msg.To == address.Undef {
		return xerrors.New("'To' address cannot be empty")
	}

	if msg.To == ZeroAddress && version >= network.Version7 {
		return xerrors.New("invalid 'To' address")
	}

	if msg.From == address.Undef {
		return xerrors.New("'From' address cannot be empty")
	}

	if msg.Value.Int == nil {
		return xerrors.New("'Value' cannot be nil")
	}

	if msg.Value.LessThan(ZeroFIL) {
		return xerrors.New("'Value' field cannot be negative")
	}

	if msg.Value.GreaterThan(TotalFilecoinInt) {
		return xerrors.New("'Value' field cannot be greater than total filecoin supply")
	}

	if msg.GasFeeCap.Int == nil {
		return xerrors.New("'GasFeeCap' cannot be nil")
	}

	if msg.GasFeeCap.LessThan(specsbig.Zero()) {
		return xerrors.New("'GasFeeCap' field cannot be negative")
	}

	if msg.GasPremium.Int == nil {
		return xerrors.New("'GasPremium' cannot be nil")
	}

	if msg.GasPremium.LessThan(specsbig.Zero()) {
		return xerrors.New("'GasPremium' field cannot be negative")
	}

	if msg.GasPremium.GreaterThan(msg.GasFeeCap) {
		return xerrors.New("'GasFeeCap' less than 'GasPremium'")
	}

	if msg.GasLimit > constants.BlockGasLimit {
		return xerrors.New("'GasLimit' field cannot be greater than a newBlock's gas limit")
	}

	// since prices might vary with time, this is technically semantic validation
	if msg.GasLimit < minGas {
		return xerrors.New("'GasLimit' field cannot be less than the cost of storing a message on chain")
	}

	return nil
}

func DecodeMessage(b []byte) (*UnsignedMessage, error) {
	var msg UnsignedMessage

	if err := msg.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	if msg.Version != MessageVersion {
		return nil, fmt.Errorf("decoded message had incorrect version (%d)", msg.Version)
	}

	return &msg, nil
}

func NewGasFeeCap(price int64) abi.TokenAmount {
	return NewAttoFIL(big.NewInt(price))
}

func NewGasPremium(price int64) abi.TokenAmount {
	return NewAttoFIL(big.NewInt(price))
}

// TxMeta tracks the merkleroots of both secp and bls messages separately
type TxMeta struct {
	BLSRoot  cid.Cid `json:"blsRoot"`
	SecpRoot cid.Cid `json:"secpRoot"`
}

// String returns a readable printing string of TxMeta
func (m TxMeta) String() string {
	return fmt.Sprintf("secp: %s, bls: %s", m.SecpRoot.String(), m.BLSRoot.String())
}

func (m *TxMeta) Cid() cid.Cid {
	b, err := m.ToStorageBlock()
	if err != nil {
		panic(err) // also maybe sketchy
	}
	return b.Cid()
}

func (m *TxMeta) ToStorageBlock() (block.Block, error) {
	var buf bytes.Buffer
	if err := m.MarshalCBOR(&buf); err != nil {
		return nil, xerrors.Errorf("failed to marshal MsgMeta: %w", err)
	}

	c, err := abi.CidBuilder.Sum(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return block.NewBlockWithCid(buf.Bytes(), c)
}

// MessageReceipt is what is returned by executing a message on the vm.
type MessageReceipt struct {
	ExitCode    exitcode.ExitCode `json:"exitCode"`
	ReturnValue []byte            `json:"return"`
	GasUsed     int64             `json:"gasUsed"`
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode, gasAmount int64) MessageReceipt {
	return MessageReceipt{
		ExitCode:    exitCode,
		ReturnValue: []byte{},
		GasUsed:     gasAmount,
	}
}

func (r *MessageReceipt) String() string {
	errStr := "(error encoding MessageReceipt)"

	js, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return errStr
	}
	return fmt.Sprintf("MessageReceipt: %s", string(js))
}
