package crypto_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/crypto"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func TestKeyInfoMarshal(t *testing.T) {
	tf.UnitTest(t)

	ki := crypto.KeyInfo{
		PrivateKey: []byte{1, 2, 3, 4},
		SigType:    crypto.SigTypeSecp256k1,
	}
	buf := new(bytes.Buffer)
	err := ki.MarshalCBOR(buf)
	assert.NoError(t, err)

	kiBack := &crypto.KeyInfo{}
	err = kiBack.UnmarshalCBOR(buf)
	assert.NoError(t, err)

	assert.Equal(t, ki.Key(), kiBack.Key())
	assert.Equal(t, ki.Type(), kiBack.Type())
	assert.True(t, ki.Equals(kiBack))
}
