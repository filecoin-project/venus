package keystore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

func makePrivateKey(t *testing.T) ci.PrivKey {
	// create a public private key pair
	sk, _, err := ci.GenerateKeyPair(ci.RSA, 1024)
	require.NoError(t, err)
	return sk

}

func TestKeystoreValidateName(t *testing.T) {
	assert := assert.New(t)

	assert.Error(validateName(""), ErrKeyFmt)
	assert.Error(validateName("."), ErrKeyFmt)
	assert.Error(validateName("/."), ErrKeyFmt)
	assert.Error(validateName("./"), ErrKeyFmt)
	assert.Error(validateName("bad/key"), ErrKeyFmt)
	assert.Error(validateName(".badkey"), ErrKeyFmt)
	assert.Error(validateName(".re/llyadkey"), ErrKeyFmt)
}
