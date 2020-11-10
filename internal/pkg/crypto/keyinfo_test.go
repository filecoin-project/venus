package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/internal/pkg/crypto"
	tf "github.com/filecoin-project/venus/internal/pkg/testhelpers/testflags"
)

func TestKeyInfoMarshal(t *testing.T) {
	tf.UnitTest(t)

	ki := crypto.KeyInfo{
		PrivateKey: []byte{1, 2, 3, 4},
		SigType:    crypto.SigTypeSecp256k1,
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
