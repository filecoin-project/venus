package proofs

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

func TestVerifyPoSt(t *testing.T) {
	assert := assert.New(t)
	challenge := []byte("anything")

	t.Run("IsPoStValidWithProver returns true with no error when the proof is valid", func(t *testing.T) {
		goodProof := PoStProof{0x3, 0x3, 0x3}
		yesMan := FakeProver{true, nil} // guaranteed to verify
		res, err := IsPoStValidWithProver(yesMan, goodProof[:], challenge)
		assert.True(res)
		assert.Nil(err)
	})

	t.Run("IsPoStValidWithProver returns false + no error when the proof is invalid", func(t *testing.T) {
		someProof := PoStProof{0x3, 0x3, 0x3}
		noMan := FakeProver{false, nil}
		res, err := IsPoStValidWithProver(noMan, someProof[:], challenge)
		assert.False(res)
		assert.NoError(err)
	})

	t.Run("IsPoStValidWithProver returns false + error if the prover errors", func(t *testing.T) {
		someProof := PoStProof{0x3, 0x3, 0x3}
		noWayMan := FakeProver{false, errors.New("Boom")}
		res, err := IsPoStValidWithProver(noWayMan, someProof[:], challenge)
		assert.False(res)
		assert.Error(err, "boom")
	})

	t.Run("IsPoStValidWithProver returns false + error when the proof has the wrong length", func(t *testing.T) {
		someProof := []byte{0x3, 0x3, 0x3}
		noMan := FakeProver{false, nil}
		res, err := IsPoStValidWithProver(noMan, someProof, challenge)
		assert.False(res)
		assert.Error(err, "proof must be 192 bytes, but is 3 bytes")
	})

}
