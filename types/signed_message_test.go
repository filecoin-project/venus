package types

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

var mockSigner, _ = NewMockSignersAndKeyInfo(10)
var newSignedMessage = NewSignedMessageForTestGetter(mockSigner)

func TestSignedMessageString(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	smsg := newSignedMessage()
	cid, err := smsg.Cid()
	require.NoError(err)

	got := smsg.String()
	assert.Contains(got, cid.String())
}

func TestSignedMessageRecover(t *testing.T) {
	assert := assert.New(t)

	smsg := newSignedMessage()

	mockRecoverer := MockRecoverer{}

	addr, err := smsg.RecoverAddress(&mockRecoverer)
	assert.NoError(err)
	assert.Equal(mockSigner.Addresses[0], addr)
}

func TestSignedMessageMarshal(t *testing.T) {
	assert := assert.New(t)

	smsg := newSignedMessage()

	marshalled, err := smsg.Marshal()
	assert.NoError(err)

	smsgBack := SignedMessage{}
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
}

func TestSignedMessageCid(t *testing.T) {
	assert := assert.New(t)

	smsg1 := newSignedMessage()
	smsg2 := newSignedMessage()

	c1, err := smsg1.Cid()
	assert.NoError(err)
	c2, err := smsg2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}
