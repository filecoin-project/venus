package proofs

import (
	"testing"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/stretchr/testify/assert"
)

func TestVerifyPoSt(t *testing.T) {
	assert := assert.New(t)
	challengeSeed := PoStChallengeSeed{1, 2, 3}

	t.Run("IsPoStValidWithVerifier returns true with no error when the proof is valid", func(t *testing.T) {
		goodProof := PoStProof{0x3, 0x3, 0x3}
		yesMan := FakeVerifier{true, nil} // guaranteed to verify
		res, err := IsPoStValidWithVerifier(yesMan, [][32]byte{}, challengeSeed, []uint64{}, goodProof)
		assert.True(res)
		assert.Nil(err)
	})

	t.Run("IsPoStValidWithVerifier returns false + no error when the proof is invalid", func(t *testing.T) {
		someProof := PoStProof{0x3, 0x3, 0x3}
		noMan := FakeVerifier{false, nil}
		res, err := IsPoStValidWithVerifier(noMan, [][32]byte{}, challengeSeed, []uint64{}, someProof)
		assert.False(res)
		assert.NoError(err)
	})

	t.Run("IsPoStValidWithVerifier returns false error if the prover errors", func(t *testing.T) {
		someProof := PoStProof{0x3, 0x3, 0x3}
		noWayMan := FakeVerifier{false, errors.New("Boom")}
		res, err := IsPoStValidWithVerifier(noWayMan, [][32]byte{}, challengeSeed, []uint64{}, someProof)
		assert.False(res)
		assert.Error(err, "boom")
	})
}
