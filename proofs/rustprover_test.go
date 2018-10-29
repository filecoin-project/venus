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

func TestRustProverSealAndUnsealSymmetry(t *testing.T) {
	require := require.New(t)

	sealed, err := ioutil.TempDir("", "sealed")
	require.NoError(err)

	staging, err := ioutil.TempDir("", "staging")
	require.NoError(err)

	proverID := createProverID()
	sectorID := createSectorID()

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
		ProverID:     proverID,
		SealedPath:   sealOutputPath,
		SectorID:     sectorID,
		Storage:      sm,
		UnsealedPath: sealInputPath,
	})
	require.NoError(err, "seal operation failed")

	ures, err := rp.Unseal(UnsealRequest{
		NumBytes:    uint64(len(bs[1])),
		OutputPath:  unsealOutputPath,
		ProverID:    proverID,
		SealedPath:  sealOutputPath,
		SectorID:    sectorID,
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

func TestRustProverFullCycle(t *testing.T) {
	require := require.New(t)

	stagingDir, err := ioutil.TempDir("", "staging")
	require.NoError(err)
	defer os.RemoveAll(stagingDir)

	sealedDir, err := ioutil.TempDir("", "sealed")
	require.NoError(err)
	defer os.RemoveAll(sealedDir)

	cases := []struct {
		name  string
		store SectorStore
	}{
		{
			name:  "NewProofTestSectorStore",
			store: NewProofTestSectorStore(stagingDir, sealedDir),
		},
		{
			name:  "NewTestSectorStore",
			store: NewTestSectorStore(stagingDir, sealedDir),
		},
	}

	for _, c := range cases {
		store := c.store

		proverID := createProverID()
		sectorID := createSectorID()

		resGetMaxUnsealedBytes, err := store.GetMaxUnsealedBytesPerSector()
		require.NoError(err)

		resNewStagingSectorAccess, err := store.NewStagingSectorAccess()
		require.NoError(err)

		resNewSealedSectorAccess, err := store.NewSealedSectorAccess()
		require.NoError(err)

		// STEP ONE: write piece-bytes to unsealed sector access

		pieceData := make([]byte, resGetMaxUnsealedBytes.NumBytes/3)
		_, err = io.ReadFull(rand.Reader, pieceData)
		require.NoError(err)

		resWriteUnsealed, err := store.WriteUnsealed(WriteUnsealedRequest{
			SectorAccess: resNewStagingSectorAccess.SectorAccess,
			Data:         pieceData,
		})
		require.NoError(err)
		require.Equal(uint64(len(pieceData)), resWriteUnsealed.NumBytesWritten)

		// STEP TWO: seal the unsealed sector

		resSeal, err := (&RustProver{}).Seal(SealRequest{
			ProverID:     proverID,
			SealedPath:   resNewSealedSectorAccess.SectorAccess,
			SectorID:     sectorID,
			Storage:      store,
			UnsealedPath: resNewStagingSectorAccess.SectorAccess,
		})
		require.NoError(err)

		// STEP THREE: verify the returned proof and commitments

		res, err := (&RustProver{}).VerifySeal(VerifySealRequest{
			CommD:     resSeal.CommD,
			CommR:     resSeal.CommR,
			CommRStar: resSeal.CommRStar,
			Proof:     resSeal.Proof,
			ProverID:  proverID,
			SectorID:  sectorID,
			Storage:   store,
		})
		require.NoError(err)
		require.True(res.IsValid)

		// STEP FOUR: unseal the sealed sector and verify that we recovered
		// our piece's bytes

		resNewStagingSectorAccess2, err := store.NewStagingSectorAccess()
		require.NoError(err)

		resUnseal, err := (&RustProver{}).Unseal(UnsealRequest{
			NumBytes:    uint64(len(pieceData)),
			OutputPath:  resNewStagingSectorAccess2.SectorAccess,
			ProverID:    proverID,
			SealedPath:  resNewSealedSectorAccess.SectorAccess,
			SectorID:    sectorID,
			StartOffset: 0,
			Storage:     store,
		})
		require.NoError(err)
		require.Equal(int(resUnseal.NumBytesWritten), len(pieceData))

		bytesWeRead, err := ioutil.ReadFile(resNewStagingSectorAccess2.SectorAccess)
		require.NoError(err)

		require.Equal(fmt.Sprintf("%08b", pieceData), fmt.Sprintf("%08b", bytesWeRead))
	}
}

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

	vres, verr := (&RustProver{}).VerifyPoST(VerifyPoSTRequest{
		Proof: gres.Proof,
	})
	require.NoError(verr)
	require.True(vres.IsValid)
}

func TestHandlesNullFaultsPtr(t *testing.T) {
	gres, gerr := (&RustProver{}).GeneratePoST(GeneratePoSTRequest{
		CommRs:        [][32]byte{},
		ChallengeSeed: [32]byte{},
	})
	require.NoError(t, gerr)

	require.Equal(t, 0, len(gres.Faults))
}

func createProverID() [31]byte {
	slice := make([]byte, 31)

	if _, err := io.ReadFull(rand.Reader, slice); err != nil {
		panic(err)
	}

	var proverID [31]byte
	copy(proverID[:], slice)

	return proverID
}

func createSectorID() [31]byte {
	slice := make([]byte, 31)

	if _, err := io.ReadFull(rand.Reader, slice); err != nil {
		panic(err)
	}

	var sectorID [31]byte
	copy(sectorID[:], slice)

	return sectorID
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
