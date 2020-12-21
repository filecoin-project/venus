package types

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-amt-ipld/v2"
	"github.com/filecoin-project/go-state-types/abi"
	tbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/enccid"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	errPkg "github.com/pkg/errors"
	typegen "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/cborutil"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/encoding"
)

const FilecoinPrecision = uint64(1_000_000_000_000_000_000)

type MessageSendSpec struct {
	MaxFee abi.TokenAmount
}

var DefaultMessageSendSpec = MessageSendSpec{
	// MaxFee of 0.1FIL
	MaxFee: abi.NewTokenAmount(int64(FilecoinPrecision) / 10),
}

func (ms *MessageSendSpec) Get() MessageSendSpec {
	if ms == nil {
		return DefaultMessageSendSpec
	}

	return *ms
}

const MessageVersion = 0

// EmptyMessagesCID is the cid of an empty collection of messages.
var EmptyMessagesCID cid.Cid

// EmptyReceiptsCID is the cid of an empty collection of receipts.
var EmptyReceiptsCID cid.Cid

// EmptyTxMetaCID is the cid of a TxMeta wrapping empty cids
var EmptyTxMetaCID cid.Cid

func FromFil(i uint64) AttoFIL {
	return tbig.Mul(tbig.NewInt(int64(i)), tbig.NewInt(int64(FilecoinPrecision)))
}

func init() {
	tmpCst := cborutil.NewIpldStore(blockstore.NewBlockstore(datastore.NewMapDatastore()))
	emptyAMTCid, err := amt.FromArray(context.Background(), tmpCst, []typegen.CBORMarshaler{})
	if err != nil {
		panic("could not create CID for empty AMT")
	}
	EmptyMessagesCID = emptyAMTCid
	EmptyReceiptsCID = emptyAMTCid
	EmptyTxMetaCID, err = tmpCst.Put(context.Background(), TxMeta{SecpRoot: enccid.NewCid(EmptyMessagesCID), BLSRoot: enccid.NewCid(EmptyMessagesCID)})
	if err != nil {
		panic("could not create CID for empty TxMeta")
	}
}

//
type ChainMsg interface {
	Cid() (cid.Cid, error)
	VMMessage() *UnsignedMessage
	ToStorageBlock() (blocks.Block, error)
	// FIXME: This is the *message* length, this name is misleading.
	ChainLength() int
}

var _ ChainMsg = &UnsignedMessage{}

// UnsignedMessage is an exchange of information between two actors modeled
// as a function call.
type UnsignedMessage struct {
	// control field for encoding struct as an array
	_ struct{} `cbor:",toarray"`

	Version int64 `json:"version"`

	To   address.Address `json:"to"`
	From address.Address `json:"from"`
	// When receiving a message from a user account the nonce in
	// the message must match the expected nonce in the from actor.
	// This prevents replay attacks.
	Nonce uint64 `json:"nonce"`

	Value AttoFIL `json:"value"`

	GasLimit   Unit    `json:"gasLimit"`
	GasFeeCap  AttoFIL `json:"gasFeeCap"`
	GasPremium AttoFIL `json:"gasPremium"`

	Method abi.MethodNum `json:"method"`
	Params []byte        `json:"params"`
}

// NewUnsignedMessage creates a new message.
func NewUnsignedMessage(from, to address.Address, nonce uint64, value AttoFIL, method abi.MethodNum, params []byte) *UnsignedMessage {
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
func NewMeteredMessage(from, to address.Address, nonce uint64, value AttoFIL, method abi.MethodNum, params []byte, gasFeeCap, gasPremium AttoFIL, limit Unit) *UnsignedMessage {
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

func (msg *UnsignedMessage) RequiredFunds() tbig.Int {
	return tbig.Mul(msg.GasFeeCap, tbig.NewInt(int64(msg.GasLimit)))
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
	data, err := encoding.Encode(msg)
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
		msg.Nonce == other.Nonce &&
		msg.Value.Equals(other.Value) &&
		msg.GasPremium.Equals(other.GasPremium) &&
		msg.GasFeeCap.Equals(other.GasFeeCap) &&
		msg.GasLimit == other.GasLimit &&
		msg.Method == other.Method &&
		bytes.Equal(msg.Params, other.Params)
}

func (msg *UnsignedMessage) ChainLength() int {
	ser, err := msg.Marshal()
	if err != nil {
		panic(err)
	}
	return len(ser)
}

func (msg *UnsignedMessage) VMMessage() *UnsignedMessage {
	return msg
}

func (msg *UnsignedMessage) ToStorageBlock() (blocks.Block, error) {
	data, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (msg *UnsignedMessage) ValidForBlockInclusion(minGas int64, version network.Version) error {
	if msg.Version != 0 {
		return xerrors.New("'Version' unsupported")
	}

	if msg.To == address.Undef {
		return xerrors.New("'To' address cannot be empty")
	}

	if msg.To == constants.ZeroAddress && version >= network.Version7 {
		return xerrors.New("invalid 'To' address")
	}

	if msg.From == address.Undef {
		return xerrors.New("'From' address cannot be empty")
	}

	if msg.Value.Int == nil {
		return xerrors.New("'Value' cannot be nil")
	}

	if msg.Value.LessThan(tbig.Zero()) {
		return xerrors.New("'Value' field cannot be negative")
	}

	if msg.Value.GreaterThan(crypto.TotalFilecoinInt) {
		return xerrors.New("'Value' field cannot be greater than total filecoin supply")
	}

	if msg.GasFeeCap.Int == nil {
		return xerrors.New("'GasFeeCap' cannot be nil")
	}

	if msg.GasFeeCap.LessThan(tbig.Zero()) {
		return xerrors.New("'GasFeeCap' field cannot be negative")
	}

	if msg.GasPremium.Int == nil {
		return xerrors.New("'GasPremium' cannot be nil")
	}

	if msg.GasPremium.LessThan(tbig.Zero()) {
		return xerrors.New("'GasPremium' field cannot be negative")
	}

	if msg.GasPremium.GreaterThan(msg.GasFeeCap) {
		return xerrors.New("'GasFeeCap' less than 'GasPremium'")
	}

	if msg.GasLimit > constants.BlockGasLimit {
		return xerrors.New("'GasLimit' field cannot be greater than a block's gas limit")
	}

	// since prices might vary with time, this is technically semantic validation
	if int64(msg.GasLimit) < minGas {
		return xerrors.New("'GasLimit' field cannot be less than the cost of storing a message on chain")
	}

	return nil
}

func DecodeMessage(b []byte) (*UnsignedMessage, error) {
	var msg UnsignedMessage

	if err := encoding.Decode(b, &msg); err != nil {
		return nil, err
	}

	if msg.Version != MessageVersion {
		return nil, fmt.Errorf("decoded message had incorrect version (%d)", msg.Version)
	}

	return &msg, nil
}

func NewGasFeeCap(price int64) AttoFIL {
	return NewAttoFIL(big.NewInt(price))
}

func NewGasPremium(price int64) AttoFIL {
	return NewAttoFIL(big.NewInt(price))
}

// TxMeta tracks the merkleroots of both secp and bls messages separately
type TxMeta struct {
	_        struct{}   `cbor:",toarray"`
	BLSRoot  enccid.Cid `json:"blsRoot"`
	SecpRoot enccid.Cid `json:"secpRoot"`
}

// String returns a readable printing string of TxMeta
func (m TxMeta) String() string {
	return fmt.Sprintf("secp: %s, bls: %s", m.SecpRoot.String(), m.BLSRoot.String())
}

// MessageReceipt is what is returned by executing a message on the vm.
type MessageReceipt struct {
	// control field for encoding struct as an array
	_           struct{}          `cbor:",toarray"`
	ExitCode    exitcode.ExitCode `json:"exitCode"`
	ReturnValue []byte            `json:"return"`
	GasUsed     Unit              `json:"gasUsed"`
}

// Failure returns with a non-zero exit code.
func Failure(exitCode exitcode.ExitCode, gasAmount Unit) MessageReceipt {
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
