package proofs

import (
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPoSTCycle(t *testing.T) {
	require := require.New(t)

	gres, gerr := (&RustProver{}).GeneratePoST(GeneratePoSTRequest{
		CommRs:        [][32]byte{createDummyCommR(), createDummyCommR()},
		ChallengeSeed: [32]byte{},
	})
	require.NoError(gerr)

	// TODO: Replace these hard-coded values (in rust-proofs) with an
	// end-to-end PoST test over a small number of replica commitments
	require.Equal("00101010", fmt.Sprintf("%08b", gres.Proof[0]))
	require.Equal(1, len(gres.Faults))
	require.Equal(uint64(0), gres.Faults[0])

	challenge := []byte("doesnt matter")
	vres, verr := IsPoStValidWithProver(&RustProver{}, gres.Proof[:], challenge)
	require.NoError(verr)
	require.True(vres)
}

func TestHandlesNullFaultsPtr(t *testing.T) {
	gres, gerr := (&RustProver{}).GeneratePoST(GeneratePoSTRequest{
		CommRs:        [][32]byte{},
		ChallengeSeed: [32]byte{},
	})
	require.NoError(t, gerr)

	require.Equal(t, 0, len(gres.Faults))
}

func createDummyCommR() [32]byte {
	slice := make([]byte, 32)

	if _, err := io.ReadFull(rand.Reader, slice); err != nil {
		panic(err)
	}

	var commR [32]byte
	copy(commR[:], slice)

	return commR
}
