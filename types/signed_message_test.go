package types

import (
	"reflect"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
)

var mockSigner = NewMockSigner(MustGenerateKeyInfo(1, GenerateKeyInfoSeed()))

func TestSignedMessageString(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	smsg := makeMessage(t, mockSigner, 42)
	cid, err := smsg.Cid()
	require.NoError(err)

	got := smsg.String()
	assert.Contains(got, cid.String())
}

func TestSignedMessageRecover(t *testing.T) {
	assert := assert.New(t)

	smsg := makeMessage(t, mockSigner, 42)

	mockRecoverer := MockRecoverer{}

	addr, err := smsg.RecoverAddress(&mockRecoverer)
	assert.NoError(err)
	assert.Equal(mockSigner.Addresses[0], addr)
}

func TestSignedMessageMarshal(t *testing.T) {
	assert := assert.New(t)

	smsg := makeMessage(t, mockSigner, 42)

	marshalled, err := smsg.Marshal()
	assert.NoError(err)

	smsgBack := SignedMessage{}
	assert.False(smsg.Equals(&smsgBack))

	err = smsgBack.Unmarshal(marshalled)
	assert.NoError(err)

	assert.Equal(smsg.To, smsgBack.To)
	assert.Equal(smsg.From, smsgBack.From)
	assert.Equal(smsg.Value, smsgBack.Value)
	assert.Equal(smsg.Method, smsgBack.Method)
	assert.Equal(smsg.Params, smsgBack.Params)
	assert.Equal(smsg.GasPrice, smsgBack.GasPrice)
	assert.Equal(smsg.GasLimit, smsgBack.GasLimit)
	assert.Equal(smsg.Signature, smsgBack.Signature)
	assert.True(smsg.Equals(&smsgBack))
}

func TestSignedMessageCid(t *testing.T) {
	assert := assert.New(t)

	smsg1 := makeMessage(t, mockSigner, 41)
	smsg2 := makeMessage(t, mockSigner, 42)

	c1, err := smsg1.Cid()
	assert.NoError(err)
	c2, err := smsg2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
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
