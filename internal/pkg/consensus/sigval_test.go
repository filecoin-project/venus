package consensus

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

type fakeStateView struct {
	keys map[address.Address]address.Address
}

func (f *fakeStateView) AccountSignerAddress(_ context.Context, a address.Address) (address.Address, error) {
	if a.Protocol() == address.SECP256K1 || a.Protocol() == address.BLS {
		return a, nil
	}
	resolved, ok := f.keys[a]
	if !ok {
		return address.Undef, fmt.Errorf("not found")
	}
	return resolved, nil

}

func TestSignMessageOk(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	idAddress := vmaddr.RequireIDAddress(t, 1)
	ms, kis := types.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	{
		v := NewSignatureValidator(&fakeStateView{}) // No resolution needed.
		msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), 0)
		smsg, err := types.NewSignedMessage(*msg, ms)
		require.NoError(t, err)
		assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	}
	{
		// Use ID address in message but sign with corresponding key address.
		state := &fakeStateView{keys: map[address.Address]address.Address{
			idAddress: keyAddr,
		}}
		v := NewSignatureValidator(state)
		msg := types.NewMeteredMessage(idAddress, idAddress, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), 0)
		msgData, err := msg.Marshal()
		require.NoError(t, err)
		sig, err := ms.SignBytes(msgData, keyAddr)
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: sig,
		}

		assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	}
}

// Signature is valid but signer does not match From Address.
func TestBadFrom(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	signer, kis := types.NewMockSignersAndKeyInfo(2)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)
	otherAddr, err := kis[1].Address()
	require.NoError(t, err)

	state := fakeStateView{}
	v := NewSignatureValidator(&state)

	// Can't use NewSignedMessage constructor as it always signs with msg.From.
	msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), types.GasUnits(0))
	bmsg, err := msg.Marshal()
	require.NoError(t, err)
	sig, err := signer.SignBytes(bmsg, otherAddr) // sign with addr != msg.From
	require.NoError(t, err)
	smsg := &types.SignedMessage{
		Message:   *msg,
		Signature: sig,
	}

	assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
}

// Signature corrupted.
func TestSignedMessageBadSignature(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	signer, kis := types.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	v := NewSignatureValidator(&fakeStateView{}) // no resolution needed
	msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), 0)
	smsg, err := types.NewSignedMessage(*msg, signer)
	require.NoError(t, err)

	assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	smsg.Signature.Data[0] = smsg.Signature.Data[0] ^ 0xFF
	assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
}

// Message corrupted.
func TestSignedMessageCorrupted(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	signer, kis := types.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	v := NewSignatureValidator(&fakeStateView{}) // no resolution needed
	msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewGasPrice(0), 0)
	smsg, err := types.NewSignedMessage(*msg, signer)
	require.NoError(t, err)

	assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	smsg.Message.CallSeqNum = uint64(42)
	assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
}
