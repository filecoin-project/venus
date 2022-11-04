// stm: #unit
package crypto_test

import (
	"bytes"
	"fmt"
	"testing"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-address"

	"crypto/rand"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/crypto"
	_ "github.com/filecoin-project/venus/pkg/crypto/bls"
	_ "github.com/filecoin-project/venus/pkg/crypto/secp"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestGenerateSecpKey(t *testing.T) {
	tf.UnitTest(t)

	token := bytes.Repeat([]byte{42}, 512)
	// stm: @CRYPTO_CRYPTO_NEW_BLS_KEY_001
	ki, err := crypto.NewSecpKeyFromSeed(bytes.NewReader(token))
	assert.NoError(t, err)
	sk := ki.Key()
	t.Logf("%x", sk)
	assert.Equal(t, len(sk), 32)

	msg := make([]byte, 32)
	for i := 0; i < len(msg); i++ {
		msg[i] = byte(i)
	}

	// stm: @CRYPTO_SIG_SIGN_001
	signature, err := crypto.Sign(msg, sk, crypto.SigTypeSecp256k1)
	assert.NoError(t, err)
	assert.Equal(t, len(signature.Data), 65)
	pk, err := crypto.ToPublic(crypto.SigTypeSecp256k1, sk)
	assert.NoError(t, err)
	addr, err := address.NewSecp256k1Address(pk)
	assert.NoError(t, err)
	t.Logf("%x", pk)
	// valid signature
	// stm: @CRYPTO_SIG_VERIFY_001
	assert.True(t, crypto.Verify(signature, addr, msg) == nil)

	// invalid signature - different message (too short)
	assert.False(t, crypto.Verify(signature, addr, msg[3:]) == nil)

	// invalid signature - different message
	msg2 := make([]byte, 32)
	copy(msg2, msg)
	msg2[0] = 42
	assert.False(t, crypto.Verify(signature, addr, msg2) == nil)

	// invalid signature - different digest
	digest2 := make([]byte, 65)
	copy(digest2, signature.Data)
	digest2[0] = 42
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: digest2}, addr, msg) == nil)

	// invalid signature - digest too short
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: signature.Data[3:]}, addr, msg) == nil)
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: signature.Data[:29]}, addr, msg) == nil)

	// invalid signature - digest too long
	digest3 := make([]byte, 70)
	copy(digest3, signature.Data)
	assert.False(t, crypto.Verify(&crypto.Signature{Type: crypto.SigTypeSecp256k1, Data: digest3}, addr, msg) == nil)
}

func TestBLSSigning(t *testing.T) {
	token := bytes.Repeat([]byte{42}, 512)
	// stm: @CRYPTO_CRYPTO_NEW_BLS_KEY_001
	ki, err := crypto.NewBLSKeyFromSeed(bytes.NewReader(token))
	assert.NoError(t, err)

	data := []byte("data to be signed")
	// stm: @CRYPTO_KEYINFO_PRIVATE_KEY_001
	privateKey := ki.Key()
	// stm: @CRYPTO_KEYINFO_PUBLIC_KEY_001
	publicKey, err := ki.PublicKey()
	assert.NoError(t, err)
	t.Logf("%x", privateKey)
	t.Logf("%x", publicKey)

	signature, err := crypto.Sign(data, privateKey[:], crypto.SigTypeBLS)
	require.NoError(t, err)

	// stm: @CRYPTO_KEYINFO_ADDRESS_001
	addr, err := ki.Address()
	require.NoError(t, err)

	err = crypto.Verify(signature, addr, data)
	require.NoError(t, err)

	// invalid signature fails
	err = crypto.Verify(&crypto.Signature{Type: crypto.SigTypeBLS, Data: signature.Data[3:]}, addr, data)
	require.Error(t, err)

	// invalid digest fails
	err = crypto.Verify(signature, addr, data[3:])
	require.Error(t, err)
}

func aggregateSignatures(sigs []*crypto.Signature) (*crypto.Signature, error) {
	sigsS := make([]ffi.Signature, len(sigs))
	for i := 0; i < len(sigs); i++ {
		copy(sigsS[i][:], sigs[i].Data[:ffi.SignatureBytes])
	}

	aggSig := ffi.Aggregate(sigsS)
	if aggSig == nil {
		if len(sigs) > 0 {
			return nil, fmt.Errorf("bls.Aggregate returned nil with %d signatures", len(sigs))
		}

		zeroSig := ffi.CreateZeroSignature()

		// Note: for blst this condition should not happen - nil should not
		// be returned
		return &crypto.Signature{
			Type: crypto.SigTypeBLS,
			Data: zeroSig[:],
		}, nil
	}
	return &crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: aggSig[:],
	}, nil
}

func TestVerifyAggregate(t *testing.T) {
	var (
		size     = 10
		messages = make([][]byte, size)
		blsSigs  = make([]*crypto.Signature, size)
		kis      = make([]*crypto.KeyInfo, size)
		pubKeys  = make([][]byte, size)
	)

	for idx := 0; idx < size; idx++ {
		ki, err := crypto.NewBLSKeyFromSeed(rand.Reader)
		assert.NoError(t, err)

		msg := make([]byte, 32)
		_, err = rand.Read(msg)
		require.NoError(t, err)

		blsSigs[idx], err = crypto.Sign(msg, ki.Key(), crypto.SigTypeBLS)
		require.NoError(t, err)

		messages[idx] = msg
		kis[idx] = &ki
		pubKeys[idx], err = ki.PublicKey()
		require.NoError(t, err)
	}

	blsSig, err := aggregateSignatures(blsSigs)
	require.NoError(t, err)

	// stm: @CRYPTO_SIG_VERIFY_AGGREGATE_001
	assert.NoError(t, crypto.VerifyAggregate(pubKeys, messages, blsSig.Data))
}
