package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
)

var mockSigner = NewMockSigner(MustGenerateKeyInfo(1, 42))

func TestSignedMessageString(t *testing.T) {
	tf.UnitTest(t)

	smsg := makeMessage(t, mockSigner, 42)
	cid, err := smsg.Cid()
	require.NoError(t, err)

	got := smsg.String()
	assert.Contains(t, got, cid.String())
}

func TestSignedMessageRecover(t *testing.T) {
	tf.UnitTest(t)

	smsg := makeMessage(t, mockSigner, 42)

	mockRecoverer := MockRecoverer{}

	addr, err := smsg.RecoverAddress(&mockRecoverer)
	assert.NoError(t, err)
	assert.Equal(t, mockSigner.Addresses[0], addr)
}

func TestSignedMessageMarshal(t *testing.T) {
	tf.UnitTest(t)

	smsg := makeMessage(t, mockSigner, 42)

	marshalled, err := smsg.Marshal()
	assert.NoError(t, err)

	smsgBack := SignedMessage{}
	assert.False(t, smsg.Equals(&smsgBack))

	err = smsgBack.Unmarshal(marshalled)
	assert.NoError(t, err)

	assert.Equal(t, smsg.To, smsgBack.To)
	assert.Equal(t, smsg.From, smsgBack.From)
	assert.Equal(t, smsg.Value, smsgBack.Value)
	assert.Equal(t, smsg.Method, smsgBack.Method)
	assert.Equal(t, smsg.Params, smsgBack.Params)
	assert.Equal(t, smsg.GasPrice, smsgBack.GasPrice)
	assert.Equal(t, smsg.GasLimit, smsgBack.GasLimit)
	assert.Equal(t, smsg.Signature, smsgBack.Signature)
	assert.True(t, smsg.Equals(&smsgBack))
}

func TestSignedMessageCid(t *testing.T) {
	tf.UnitTest(t)

	smsg1 := makeMessage(t, mockSigner, 41)
	smsg2 := makeMessage(t, mockSigner, 42)

	c1, err := smsg1.Cid()
	assert.NoError(t, err)
	c2, err := smsg2.Cid()
	assert.NoError(t, err)

	assert.NotEqual(t, c1.String(), c2.String())
}

func makeMessage(t *testing.T, signer MockSigner, nonce uint64) *SignedMessage {
	newAddr, err := address.NewActorAddress([]byte("receiver"))
	require.NoError(t, err)

	msg := NewMessage(
		signer.Addresses[0],
		newAddr,
		nonce,
		NewAttoFILFromFIL(2),
		"method",
		[]byte("params"))
	smsg, err := NewSignedMessage(*msg, &signer, NewGasPrice(1000), NewGasUnits(100))
	require.NoError(t, err)

	// This check requests that you add a non-zero value for new fields above,
	// then update the field count below.
	require.Equal(t, 2, reflect.TypeOf(*smsg).NumField())

	return smsg
}
