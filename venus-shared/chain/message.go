package chain

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/venus-shared/chain/params"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

const MessageVersion = 0

func DecodeMessage(b []byte) (*Message, error) {
	var msg Message
	if err := msg.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	if msg.Version != MessageVersion {
		return nil, fmt.Errorf("decoded message had incorrect version (%d)", msg.Version)
	}

	return &msg, nil
}

type Message struct {
	Version uint64

	To   address.Address
	From address.Address
	// When receiving a message from a user account the nonce in
	// the message must match the expected nonce in the from actor.
	// This prevents replay attacks.
	Nonce uint64

	Value abi.TokenAmount

	GasLimit   int64
	GasFeeCap  abi.TokenAmount
	GasPremium abi.TokenAmount

	Method abi.MethodNum
	Params []byte
}

func (m *Message) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := m.MarshalCBOR(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (m *Message) SerializeWithCid() (cid.Cid, []byte, error) {
	data, err := m.Serialize()
	if err != nil {
		return cid.Undef, nil, err
	}

	c, err := abi.CidBuilder.Sum(data)
	if err != nil {
		return cid.Undef, nil, err
	}

	return c, data, nil
}

func (m *Message) ToStorageBlock() (blocks.Block, error) {
	c, data, err := m.SerializeWithCid()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, c)
}

func (m *Message) Cid() cid.Cid {
	c, _, err := m.SerializeWithCid()
	if err != nil {
		panic(err)
	}

	return c
}

func (m *Message) String() string {
	errStr := "(error encoding Message)"
	c, _, err := m.SerializeWithCid()
	if err != nil {
		return errStr
	}

	js, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return errStr
	}

	return fmt.Sprintf("Message cid=[%v]: %s", c, string(js))
}

func (m *Message) ChainLength() int {
	ser, err := m.Serialize()
	if err != nil {
		panic(err)
	}

	return len(ser)
}

func (m *Message) Equals(o *Message) bool {
	return m.Cid() == o.Cid()
}

func (m *Message) EqualCall(o *Message) bool {
	m1 := *m
	m2 := *o

	m1.GasLimit, m2.GasLimit = 0, 0
	m1.GasFeeCap, m2.GasFeeCap = bigZero, bigZero
	m1.GasPremium, m2.GasPremium = bigZero, bigZero

	return (&m1).Equals(&m2)
}

func (m *Message) ValidForBlockInclusion(minGas int64, version network.Version) error {
	if m.Version != 0 {
		return fmt.Errorf("'Version' unsupported")
	}

	if m.To == address.Undef {
		return fmt.Errorf("'To' address cannot be empty")
	}

	if m.To == ZeroAddress && version >= network.Version7 {
		return fmt.Errorf("invalid 'To' address")
	}

	if m.From == address.Undef {
		return fmt.Errorf("'From' address cannot be empty")
	}

	if m.Value.Int == nil {
		return fmt.Errorf("'Value' cannot be nil")
	}

	if m.Value.LessThan(bigZero) {
		return fmt.Errorf("'Value' field cannot be negative")
	}

	if m.Value.GreaterThan(TotalFilecoinInt) {
		return fmt.Errorf("'Value' field cannot be greater than total filecoin supply")
	}

	if m.GasFeeCap.Int == nil {
		return fmt.Errorf("'GasFeeCap' cannot be nil")
	}

	if m.GasFeeCap.LessThan(bigZero) {
		return fmt.Errorf("'GasFeeCap' field cannot be negative")
	}

	if m.GasPremium.Int == nil {
		return fmt.Errorf("'GasPremium' cannot be nil")
	}

	if m.GasPremium.LessThan(bigZero) {
		return fmt.Errorf("'GasPremium' field cannot be negative")
	}

	if m.GasPremium.GreaterThan(m.GasFeeCap) {
		return fmt.Errorf("'GasFeeCap' less than 'GasPremium'")
	}

	if m.GasLimit > params.BlockGasLimit {
		return fmt.Errorf("'GasLimit' field cannot be greater than a block's gas limit")
	}

	// since prices might vary with time, this is technically semantic validation
	if m.GasLimit < minGas {
		return fmt.Errorf("'GasLimit' field cannot be less than the cost of storing a message on chain %d < %d", m.GasLimit, minGas)
	}

	return nil
}
