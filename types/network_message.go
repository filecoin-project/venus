package types

import (
	"math/big"

	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
)

// GasUnits represents number of units of gas consumed
type GasUnits = Uint64

// MaxGasUnits is a convenience value for when we want to guarantee a direct message does not fail due
//    to lack of gas
const MaxGasUnits = ^uint64(0)

func init() {
	cbor.RegisterCborType(NetworkMessage{})
}

// NetworkMessage contains a message and its associated gas price and gas limit
type NetworkMessage struct {
	Message
	GasPrice AttoFIL  `json:"gasPrice"`
	GasLimit GasUnits `json:"gasLimit"`
}

// Unmarshal a message from the given bytes.
func (msg *NetworkMessage) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, msg)
}

// Marshal the message into bytes.
func (msg *NetworkMessage) Marshal() ([]byte, error) {
	return cbor.DumpObject(msg)
}

// NewNetworkMessage accepts a message `msg`, a gas price `gasPrice` and a `gasLimit`.
// It returns a network message with the message, gas price and gas limit included.
func NewNetworkMessage(msg Message, gasPrice AttoFIL, gasLimit GasUnits) (*NetworkMessage) {
	return &NetworkMessage{
		Message:  msg,
		GasPrice: gasPrice,
		GasLimit: gasLimit,
	}
}

// NewGasPrice constructs a gas price (in AttoFIL) from the given number.
func NewGasPrice(price int64) AttoFIL {
	return *NewAttoFIL(big.NewInt(price))
}

// NewGasUnits constructs a new GasUnits from the given number.
func NewGasUnits(cost uint64) GasUnits {
	return Uint64(cost)
}
