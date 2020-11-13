package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/internal/pkg/types"
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

	ms, kis := types.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	t.Run("no resolution", func(t *testing.T) {
		v := NewSignatureValidator(&fakeStateView{}) // No resolution needed.
		msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewAttoFILFromFIL(0), types.NewAttoFILFromFIL(0), 1)
		smsg, err := types.NewSignedMessage(ctx, *msg, ms)
		require.NoError(t, err)
		assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	})
	t.Run("resolution required", func(t *testing.T) {
		idAddress := types.RequireIDAddress(t, 1)
		// Use ID address in message but sign with corresponding key address.
		state := &fakeStateView{keys: map[address.Address]address.Address{
			idAddress: keyAddr,
		}}
		v := NewSignatureValidator(state)
		msg := types.NewMeteredMessage(idAddress, idAddress, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewAttoFILFromFIL(0), types.NewAttoFILFromFIL(0), 1)
		msgCid, err := msg.Cid()
		require.NoError(t, err)
		sig, err := ms.SignBytes(ctx, msgCid.Bytes(), keyAddr)
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: sig,
		}

		assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	})
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

	t.Run("no resolution", func(t *testing.T) {
		v := NewSignatureValidator(&fakeStateView{})

		// Can't use NewSignedMessage constructor as it always signs with msg.From.
		msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewAttoFILFromFIL(0), types.NewAttoFILFromFIL(0), 1)
		bmsg, err := msg.Marshal()
		require.NoError(t, err)
		sig, err := signer.SignBytes(ctx, bmsg, otherAddr) // sign with addr != msg.From
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: sig,
		}
		assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
	})
	t.Run("resolution required", func(t *testing.T) {
		idAddress := types.RequireIDAddress(t, 1)
		// Use ID address in message but sign with corresponding key address.
		state := &fakeStateView{keys: map[address.Address]address.Address{
			idAddress: keyAddr,
		}}
		v := NewSignatureValidator(state)

		// Can't use NewSignedMessage constructor as it always signs with msg.From.
		msg := types.NewMeteredMessage(idAddress, idAddress, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewAttoFILFromFIL(0), types.NewAttoFILFromFIL(0), 1)
		bmsg, err := msg.Marshal()
		require.NoError(t, err)
		sig, err := signer.SignBytes(ctx, bmsg, otherAddr) // sign with addr != msg.From (resolved)
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: sig,
		}
		assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
	})
}

// Signature corrupted.
func TestSignedMessageBadSignature(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	signer, kis := types.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	v := NewSignatureValidator(&fakeStateView{}) // no resolution needed
	msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewAttoFILFromFIL(0), types.NewAttoFILFromFIL(0), 1)
	smsg, err := types.NewSignedMessage(ctx, *msg, signer)
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
	msg := types.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroAttoFIL, builtin.MethodSend, nil, types.NewAttoFILFromFIL(0), types.NewAttoFILFromFIL(0), 1)
	smsg, err := types.NewSignedMessage(ctx, *msg, signer)
	require.NoError(t, err)

	assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	smsg.Message.CallSeqNum = uint64(42)
	assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
}
