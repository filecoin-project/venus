package crypto_test

import (
	"math/rand"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	"github.com/filecoin-project/go-filecoin/crypto"
)

func TestGenerateKey(t *testing.T) {
	assert := assert.New(t)
	rand.Seed(time.Now().UnixNano())

	sk, err := crypto.GenerateKey()
	assert.NoError(err)

	assert.Equal(len(sk), 32)

	msg := make([]byte, 32)
	for i := 0; i < len(msg); i++ {
		msg[i] = byte(i)
	}

	digest, err := crypto.Sign(sk, msg)
	assert.NoError(err)
	assert.Equal(len(digest), 65)
	pk := crypto.PublicKey(sk)

	// valid signature
	assert.True(crypto.Verify(pk, msg, digest))

	// invalid signature - different message (too short)
	assert.False(crypto.Verify(pk, msg[3:], digest))

	// invalid signature - different message
	msg2 := make([]byte, 32)
	copy(msg2, msg)
	rand.Shuffle(len(msg2), func(i, j int) { msg2[i], msg2[j] = msg2[j], msg2[i] })
	assert.False(crypto.Verify(pk, msg2, digest))

	// invalid signature - different digest
	digest2 := make([]byte, 65)
	copy(digest2, digest)
	rand.Shuffle(len(digest2), func(i, j int) { digest2[i], digest2[j] = digest2[j], digest2[i] })
	assert.False(crypto.Verify(pk, msg, digest2))

	// invalid signature - digest too short
	assert.False(crypto.Verify(pk, msg, digest[3:]))
	assert.False(crypto.Verify(pk, msg, digest[:29]))

	// invalid signature - digest too long
	digest3 := make([]byte, 70)
	copy(digest3, digest)
	assert.False(crypto.Verify(pk, msg, digest3))

	recovered, err := crypto.EcRecover(msg, digest)
	assert.NoError(err)
	assert.Equal(recovered, crypto.PublicKey(sk))
}
