package proofs

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRustProverRoundTrip(t *testing.T) {
	require := require.New(t)

	p := &RustProver{}

	tmpFile, err := ioutil.TempFile("", "")
	require.NoError(err, "error creating temp (input) file")
	defer os.Remove(tmpFile.Name())

	srcPath := tmpFile.Name()
	dstPath := fmt.Sprintf("%s42", srcPath)

	proverID := make([]uint8, 31)
	for i := 0; i < 31; i++ {
		proverID[i] = uint8(i)
	}

	challengeSeed := make([]uint8, 32)
	for i := 0; i < 32; i++ {
		challengeSeed[i] = uint8(i)
	}

	randomSeed := make([]uint8, 32)
	for i := 31; i >= 0; i-- {
		randomSeed[i] = uint8(i)
	}

	sres, err := p.Seal(SealRequest{
		UnsealedPath:  srcPath,
		SealedPath:    dstPath,
		ChallengeSeed: challengeSeed,
		ProverID:      proverID,
		RandomSeed:    randomSeed,
	})
	require.NoError(err, "Seal() operation failed")

	_, err = os.Stat(dstPath)
	require.NoError(err, "Seal() operation didn't create sealed sector-file %s", dstPath)

	expected := "12345678901234567890123456789012"
	require.Equal(expected, string(sres.Commitments.CommR), "incorrect replica commitment")

	expected = "09876543210987654321098765432109"
	require.Equal(expected, string(sres.Commitments.CommD), "incorrect data commitment")

	err = p.VerifySeal(VerifySealRequest{
		Commitments: CommitmentPair{
			CommR: sres.Commitments.CommR,
			CommD: sres.Commitments.CommD,
		},
	})
	require.NoError(err, "VerifySeal() operation failed")
}

func TestStatusCodeToErrorStringMarshal(t *testing.T) {
	require := require.New(t)

	p := &RustProver{}

	err := p.VerifySeal(VerifySealRequest{
		Commitments: CommitmentPair{
			CommR: make([]byte, 32),
			CommD: make([]byte, 32),
		},
	})
	require.Error(err)

	expected := "invalid replica and/or data commitment"
	require.Equal(expected, err.Error(), "received the wrong error")
}

func TestRustProverSealAndUnsealSymmetry(t *testing.T) {
	require := require.New(t)

	p := &RustProver{}

	tmpFile, err := ioutil.TempFile("", "")
	require.NoError(err, "error creating temp (input) file")
	defer os.Remove(tmpFile.Name())

	bs := make([][]byte, 3)
	bs[0] = []byte("foo")
	bs[1] = []byte("risk")
	bs[2] = []byte("xylon")

	for _, b := range bs {
		tmpFile.Write(b)
	}

	sealInputPath := tmpFile.Name()
	sealOutputPath := fmt.Sprintf("%s_sealed", sealInputPath)
	unsealOutputPath := fmt.Sprintf("%s_unsealed", sealOutputPath)
	defer os.Remove(sealOutputPath)

	_, err = p.Seal(SealRequest{
		UnsealedPath:  sealInputPath,
		SealedPath:    sealOutputPath,
		ChallengeSeed: make([]byte, 32),
		ProverID:      make([]byte, 31),
		RandomSeed:    make([]byte, 32),
	})
	require.NoError(err, "seal operation failed")

	ures, err := p.Unseal(UnsealRequest{
		SealedPath:  sealOutputPath,
		OutputPath:  unsealOutputPath,
		StartOffset: uint64(len(bs[0])),
		NumBytes:    uint64(len(bs[1])),
	})
	require.NoError(err, "unseal operation failed")
	require.Equal(uint64(len(bs[1])), ures.NumBytesWritten)

	bytes, err := ioutil.ReadFile(unsealOutputPath)
	require.NoError(err)

	// should have respected offset and number-of-bytes
	require.Equal("risk", string(bytes))
}
