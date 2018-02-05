package types

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransactionMarshalRoundTrip(t *testing.T) {
	tx := &Transaction{
		To:        "john",
		From:      "hank",
		Value:     big.NewInt(15557),
		Signature: []byte("hank"),
	}

	data, err := tx.Encode()
	assert.NoError(t, err)

	out, err := DecodeTransaction(data)
	assert.NoError(t, err)

	assert.True(t, tx.Equals(out))
}
