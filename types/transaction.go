package types

import (
	"bytes"
	"fmt"
	"math/big"

	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
)

func init() {
	cbor.RegisterCborType(Transaction{})
}

type Transaction struct {
	To    Address
	From  Address
	Value *big.Int

	Signature []byte
}

var ErrInvalidSignature = fmt.Errorf("invalid signature")

func (tx *Transaction) ValidateSignature() error {
	// TODO: actual cryptographic signatures
	if string(tx.From) != string(tx.Signature) {
		return ErrInvalidSignature
	}

	return nil
}

func (tx *Transaction) Equals(o *Transaction) bool {
	return tx.From == o.From &&
		tx.To == o.To &&
		tx.Value.Cmp(o.Value) == 0 &&
		bytes.Equal(tx.Signature, o.Signature)
}

// TODO: need nicer interfaces from cbor ipld lib. Streaming interfaces are key
func (tx *Transaction) Encode() ([]byte, error) {
	return cbor.DumpObject(tx)
}

func DecodeTransaction(b []byte) (*Transaction, error) {
	var tx Transaction
	if err := cbor.DecodeInto(b, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}
