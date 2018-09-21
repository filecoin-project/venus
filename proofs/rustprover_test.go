package proofs

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRustProverRoundTrip(t *testing.T) {
	require := require.New(t)

	sealed, err := ioutil.TempDir("", "sealed")
	require.NoError(err)

	staging, err := ioutil.TempDir("", "staging")
	require.NoError(err)

	rp := &RustProver{}
	sm := NewProofTestSectorStore(staging, sealed)

	tmpFile, err := ioutil.TempFile("", "")
	require.NoError(err, "error creating temp (input) file")
	defer os.Remove(tmpFile.Name())

	srcPath := tmpFile.Name()
	dstPath := fmt.Sprintf("%s42", srcPath)

	sres, err := rp.Seal(SealRequest{
		ProverID:     [31]byte{},
		SealedPath:   dstPath,
		SectorID:     [31]byte{},
		Storage:      sm,
		UnsealedPath: srcPath,
	})
	require.NoError(err, "Seal() operation failed")

	_, err = os.Stat(dstPath)
	require.NoError(err, "Seal() operation didn't create sealed sector-file %s", dstPath)

	err = rp.VerifySeal(VerifySealRequest{
		CommD:    sres.CommD,
		CommR:    sres.CommR,
		Proof:    sres.Proof,
		ProverID: [31]byte{},
		SectorID: [31]byte{},
		Storage:  sm,
	})
	require.NoError(err, "VerifySeal() operation failed")
}

func TestStatusCodeToErrorStringMarshal(t *testing.T) {
	require := require.New(t)

	sealed, err := ioutil.TempDir("", "sealed")
	require.NoError(err)

	staging, err := ioutil.TempDir("", "staging")
	require.NoError(err)

	rp := &RustProver{}
	sm := NewProofTestSectorStore(staging, sealed)

	err = rp.VerifySeal(VerifySealRequest{
		CommD:    [32]byte{},
		CommR:    [32]byte{},
		Proof:    [192]byte{},
		ProverID: [31]byte{},
		SectorID: [31]byte{},
		Storage:  sm,
	})

	expected := "unhandled verify_seal error"
	require.Equal(expected, err.Error(), "received the wrong error")
}

func TestRustProverSealAndUnsealSymmetry(t *testing.T) {
	require := require.New(t)

	sealed, err := ioutil.TempDir("", "sealed")
	require.NoError(err)

	staging, err := ioutil.TempDir("", "staging")
	require.NoError(err)

	rp := &RustProver{}
	sm := NewProofTestSectorStore(staging, sealed)

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

	_, err = rp.Seal(SealRequest{
		ProverID:     [31]byte{},
		SealedPath:   sealOutputPath,
		SectorID:     [31]byte{},
		Storage:      sm,
		UnsealedPath: sealInputPath,
	})
	require.NoError(err, "seal operation failed")

	ures, err := rp.Unseal(UnsealRequest{
		NumBytes:    uint64(len(bs[1])),
		OutputPath:  unsealOutputPath,
		ProverID:    [31]byte{},
		SealedPath:  sealOutputPath,
		SectorID:    [31]byte{},
		StartOffset: uint64(len(bs[0])),
		Storage:     sm,
	})
	require.NoError(err, "unseal operation failed")
	require.Equal(uint64(len(bs[1])), ures.NumBytesWritten)

	bytes, err := ioutil.ReadFile(unsealOutputPath)
	require.NoError(err)

	// should have respected offset and number-of-bytes
	require.Equal("risk", string(bytes))
}

func TestPoSTCycle(t *testing.T) {
	require := require.New(t)

	sealed, err := ioutil.TempDir("", "sealed")
	require.NoError(err)

	staging, err := ioutil.TempDir("", "staging")
	require.NoError(err)

	rp := &RustProver{}
	sm := NewProofTestSectorStore(staging, sealed)

	res, err := rp.GeneratePoST(GeneratePoSTRequest{
		CommRs:        [][32]byte{createDummyCommR(), createDummyCommR()},
		ChallengeSeed: [32]byte{},
		Storage:       sm,
	})
	require.NoError(err)

	// TODO: Replace these hard-coded values (in rust-proofs) with an
	// end-to-end PoST test over a small number of replica commitments
	require.Equal("00101010", fmt.Sprintf("%08b", res.Proof[0]))
	require.Equal(1, len(res.Faults))
	require.Equal(0, int(res.Faults[0]))

	err = rp.VerifyPoST(VerifyPoSTRequest{
		Proof:   res.Proof,
		Storage: sm,
	})
	require.NoError(err)
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
