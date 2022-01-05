package state_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/venus/pkg/testhelpers"

	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"

	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
)

type fakeStateView struct {
	keys map[address.Address]address.Address
}

func (f *fakeStateView) ResolveToKeyAddr(_ context.Context, a address.Address) (address.Address, error) {
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

	ms, kis := testhelpers.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	t.Run("no resolution", func(t *testing.T) {
		v := state.NewSignatureValidator(&fakeStateView{}) // No resolution needed.
		msg := testhelpers.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroFIL, builtin.MethodSend, nil, types.FromFil(0), types.FromFil(0), 1)
		smsg, err := testhelpers.NewSignedMessage(ctx, *msg, ms)
		require.NoError(t, err)
		assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	})
	t.Run("resolution required", func(t *testing.T) {
		idAddress := testhelpers.RequireIDAddress(t, 1)
		// Use ID address in message but sign with corresponding key address.
		stateView := &fakeStateView{keys: map[address.Address]address.Address{
			idAddress: keyAddr,
		}}
		v := state.NewSignatureValidator(stateView)
		msg := testhelpers.NewMeteredMessage(idAddress, idAddress, 1, types.ZeroFIL, builtin.MethodSend, nil, types.FromFil(0), types.FromFil(0), 1)
		msgCid := msg.Cid()
		sig, err := ms.SignBytes(ctx, msgCid.Bytes(), keyAddr)
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: *sig,
		}

		assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	})
}

// Signature is valid but signer does not match From address.
func TestBadFrom(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	signer, kis := testhelpers.NewMockSignersAndKeyInfo(2)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)
	otherAddr, err := kis[1].Address()
	require.NoError(t, err)

	t.Run("no resolution", func(t *testing.T) {
		v := state.NewSignatureValidator(&fakeStateView{})

		// Can't use NewSignedMessage constructor as it always signs with msg.From.
		msg := testhelpers.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroFIL, builtin.MethodSend, nil, types.FromFil(0), types.FromFil(0), 1)
		buf := new(bytes.Buffer)
		err = msg.MarshalCBOR(buf)
		require.NoError(t, err)
		sig, err := signer.SignBytes(ctx, buf.Bytes(), otherAddr) // sign with addr != msg.From
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: *sig,
		}
		assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
	})
	t.Run("resolution required", func(t *testing.T) {
		idAddress := testhelpers.RequireIDAddress(t, 1)
		// Use ID address in message but sign with corresponding key address.
		stateView := &fakeStateView{keys: map[address.Address]address.Address{
			idAddress: keyAddr,
		}}
		v := state.NewSignatureValidator(stateView)

		// Can't use NewSignedMessage constructor as it always signs with msg.From.
		msg := testhelpers.NewMeteredMessage(idAddress, idAddress, 1, types.ZeroFIL, builtin.MethodSend, nil, types.FromFil(0), types.FromFil(0), 1)
		buf := new(bytes.Buffer)
		err = msg.MarshalCBOR(buf)
		require.NoError(t, err)
		sig, err := signer.SignBytes(ctx, buf.Bytes(), otherAddr) // sign with addr != msg.From (resolved)
		require.NoError(t, err)
		smsg := &types.SignedMessage{
			Message:   *msg,
			Signature: *sig,
		}
		assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
	})
}

// Signature corrupted.
func TestSignedMessageBadSignature(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()

	signer, kis := testhelpers.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	v := state.NewSignatureValidator(&fakeStateView{}) // no resolution needed
	msg := testhelpers.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroFIL, builtin.MethodSend, nil, types.FromFil(0), types.FromFil(0), 1)
	smsg, err := testhelpers.NewSignedMessage(ctx, *msg, signer)
	require.NoError(t, err)

	assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	smsg.Signature.Data[0] = smsg.Signature.Data[0] ^ 0xFF
	assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
}

// Message corrupted.
func TestSignedMessageCorrupted(t *testing.T) {
	tf.UnitTest(t)

	ctx := context.Background()

	signer, kis := testhelpers.NewMockSignersAndKeyInfo(1)
	keyAddr, err := kis[0].Address()
	require.NoError(t, err)

	v := state.NewSignatureValidator(&fakeStateView{}) // no resolution needed
	msg := testhelpers.NewMeteredMessage(keyAddr, keyAddr, 1, types.ZeroFIL, builtin.MethodSend, nil, types.FromFil(0), types.FromFil(0), 1)
	smsg, err := testhelpers.NewSignedMessage(ctx, *msg, signer)
	require.NoError(t, err)

	assert.NoError(t, v.ValidateMessageSignature(ctx, smsg))
	smsg.Message.Nonce = uint64(42)
	assert.Error(t, v.ValidateMessageSignature(ctx, smsg))
}
