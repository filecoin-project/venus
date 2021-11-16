package chain

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
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
	m1.GasFeeCap, m2.GasFeeCap = big.Zero(), big.Zero()
	m1.GasPremium, m2.GasPremium = big.Zero(), big.Zero()

	return (&m1).Equals(&m2)
}
