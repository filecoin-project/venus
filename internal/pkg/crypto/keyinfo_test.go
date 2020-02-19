package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestKeyInfoMarshal(t *testing.T) {
	tf.UnitTest(t)

	ki := crypto.KeyInfo{
		PrivateKey:  []byte{1, 2, 3, 4},
		CryptSystem: crypto.SECP256K1,
	}

	marshaled, err := ki.Marshal()
	assert.NoError(t, err)

	kiBack := &crypto.KeyInfo{}
	err = kiBack.Unmarshal(marshaled)
	assert.NoError(t, err)

	assert.Equal(t, ki.Key(), kiBack.Key())
	assert.Equal(t, ki.Type(), kiBack.Type())
	assert.True(t, ki.Equals(kiBack))
}

func TestBLSPublicKey(t *testing.T) {
	tf.UnitTest(t)
	t.Skip("Dragons: BLS is broken")

	testKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	ki := &KeyInfo{
		PrivateKey:  testKey,
		CryptSystem: BLS,
	}
	ki.PublicKey()
}
