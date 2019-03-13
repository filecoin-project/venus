package testing

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

// MaxTimeToSealASector represents the maximum amount of time the test should
// wait for a sector to be sealed. Seal performance varies depending on the
// computer, so we need to select a value which works for slow (CircleCI OSX
// build containers) and fast (developer machines) alike.
const MaxTimeToSealASector = time.Second * 360

// MaxTimeToGenerateSectorPoSt represents the maximum amount of time the test
// should wait for a proof-of-spacetime to be generated for a sector.
const MaxTimeToGenerateSectorPoSt = time.Second * 360

func TestSectorBuilder(t *testing.T) {
	if os.Getenv("FILECOIN_RUN_SECTOR_BUILDER_TESTS") != "true" {
		t.SkipNow()
	}
	t.Run("concurrent AddPiece and SealAllStagedSectors", func(t *testing.T) {
		h := NewBuilder(t).Build()
		defer h.Close()

		// stringify the content identifiers to make them easily
		// sortable later
		sealedPieceCidCh := make(chan string)
		addedPieceCidCh := make(chan string)
		errs := make(chan error)

		go func() {
			for val := range h.SectorBuilder.SectorSealResults() {
				if val.SealingErr != nil {
					errs <- val.SealingErr
				} else if val.SealingResult != nil {
					for _, pieceInfo := range val.SealingResult.Pieces {
						sealedPieceCidCh <- pieceInfo.Ref.String()
					}
				}
			}
		}()

		autoSealsToSchedule := 5
		for i := 0; i < autoSealsToSchedule; i++ {
			go func(n int) {
				time.Sleep(time.Second * time.Duration(n))
				h.SectorBuilder.SealAllStagedSectors(context.Background())
			}(i)
		}

		piecesToSeal := 10
		for i := 0; i < piecesToSeal; i++ {
			go func() {
				_, pieceCid, err := h.AddPiece(context.Background(), RequireRandomBytes(t, h.MaxBytesPerSector/3))
				if err != nil {
					errs <- err
				} else {
					addedPieceCidCh <- pieceCid.String()
				}
			}()
		}

		var addedPieceCids []string
		var sealedPieceCids []string

		// Wait for a bit of time for the various seal() ops to complete and
		// capture the cids of added pieces for comparison with the cids of
		// sealed pieces. At most we should seal 10 times (once per piece).
		timeout := time.After(MaxTimeToSealASector * 10)
		for {
			if piecesToSeal == 0 {
				break
			}
			select {
			case err := <-errs:
				require.NoError(t, err)
			case pieceCid := <-addedPieceCidCh:
				addedPieceCids = append(addedPieceCids, pieceCid)
			case pieceCid := <-sealedPieceCidCh:
				sealedPieceCids = append(sealedPieceCids, pieceCid)
				piecesToSeal--
			case <-timeout:
				t.Fatalf("timed out waiting for seal ops to complete (%d remaining)", piecesToSeal)
			}
		}

		// wait around for a few more seconds to ensure that there
		// weren't any superfluous seal() calls lingering
		timeout = time.After(5 * time.Second)
	Loop:
		for {
			select {
			case err := <-errs:
				require.NoError(t, err)
			case <-addedPieceCidCh:
				t.Fatal("should not have added any more pieces")
			case <-sealedPieceCidCh:
				t.Fatal("should not have sealed any more pieces")
			case <-timeout:
				break Loop // I've always dreamt of using GOTO
			}
		}

		sort.Strings(addedPieceCids)
		sort.Strings(sealedPieceCids)

		require.Equal(t, addedPieceCids, sealedPieceCids)
	})

	t.Run("concurrent writes", func(t *testing.T) {
		h := NewBuilder(t).Build()
		defer h.Close()

		// CIDs will be added to this map when given to the SectorBuilder and
		// removed when the CID has been sealed into a sector.
		pieceCidSet := sync.Map{}

		done := make(chan cid.Cid)
		errs := make(chan error)

		go func() {
			for val := range h.SectorBuilder.SectorSealResults() {
				if val.SealingErr != nil {
					errs <- val.SealingErr
				} else if val.SealingResult != nil {
					for _, pieceInfo := range val.SealingResult.Pieces {
						done <- pieceInfo.Ref
					}
				}
			}
		}()

		piecesToSeal := 5
		for i := 0; i < piecesToSeal; i++ {
			go func() {
				_, pieceCid, err := h.AddPiece(context.Background(), RequireRandomBytes(t, h.MaxBytesPerSector))
				if err != nil {
					errs <- err
				} else {
					pieceCidSet.Store(pieceCid.String(), true)
				}
			}()
		}

		// Sealing a small sector can take 180+ seconds on a MacBook Pro i7.
		// In the worst-case scenario, we seal 5 times (once per piece).
		timeout := time.After(MaxTimeToSealASector * 5)
		for {
			if piecesToSeal == 0 {
				break
			}
			select {
			case err := <-errs:
				require.NoError(t, err)
			case sealed := <-done:
				pieceCidSet.Delete(sealed.String())
				piecesToSeal--
			case <-timeout:
				t.Fatalf("timed out waiting for seal ops to complete (%d remaining)", piecesToSeal)
			}
		}

		pieceCidSet.Range(func(key, value interface{}) bool {
			t.Fatalf("should have removed each piece from set as they were sealed (found %s)", key)
			return false
		})
	})

	t.Run("add, seal, verify, and read user piece-bytes", func(t *testing.T) {
		h := NewBuilder(t).Build()
		defer h.Close()

		inputBytes := RequireRandomBytes(t, h.MaxBytesPerSector)
		info, err := h.CreatePieceInfo(inputBytes)
		require.NoError(t, err)

		sectorID, err := h.SectorBuilder.AddPiece(context.Background(), info)
		require.NoError(t, err)

		// Sealing can take 180+ seconds on an i7 MacBook Pro. We are sealing
		// but one sector in this test.
		timeout := time.After(MaxTimeToSealASector)

		select {
		case val := <-h.SectorBuilder.SectorSealResults():
			require.NoError(t, val.SealingErr)
			require.Equal(t, sectorID, val.SealingResult.SectorID)

			res, err := (&proofs.RustVerifier{}).VerifySeal(proofs.VerifySealRequest{
				CommD:     val.SealingResult.CommD,
				CommR:     val.SealingResult.CommR,
				CommRStar: val.SealingResult.CommRStar,
				Proof:     val.SealingResult.Proof,
				ProverID:  sectorbuilder.AddressToProverID(h.MinerAddr),
				SectorID:  sectorbuilder.SectorIDToBytes(val.SealingResult.SectorID),
				StoreType: h.SectorConfig,
			})
			require.NoError(t, err)
			require.True(t, res.IsValid)
		case <-timeout:
			t.Fatalf("timed out waiting for seal to complete")
		}

		reader, err := h.SectorBuilder.ReadPieceFromSealedSector(info.Ref)
		require.NoError(t, err)

		outputBytes, err := ioutil.ReadAll(reader)
		require.NoError(t, err)

		require.Equal(t, hex.EncodeToString(inputBytes), hex.EncodeToString(outputBytes))
	})

	t.Run("sector builder resumes polling for staged sectors even after a restart", func(t *testing.T) {
		stagingDir, err := ioutil.TempDir("", "staging")
		if err != nil {
			panic(err)
		}

		sealedDir, err := ioutil.TempDir("", "staging")
		if err != nil {
			panic(err)
		}

		hA := NewBuilder(t).StagingDir(stagingDir).SealedDir(sealedDir).Build()
		defer hA.Close()

		// holds id of each sector we expect to see sealed
		sectorIDSet := sync.Map{}

		// SectorBuilder begins polling for SectorIDA seal-status
		sectorIDA, _, errA := hA.AddPiece(context.Background(), RequireRandomBytes(t, hA.MaxBytesPerSector-10))
		require.NoError(t, errA)
		sectorIDSet.Store(sectorIDA, true)

		// create new SectorBuilder which should start with a poller pre-seeded
		// with state from previous SectorBuilder
		hB := NewBuilder(t).StagingDir(stagingDir).SealedDir(sealedDir).Build()
		defer hB.Close()

		// second SectorBuilder begins polling for SectorIDB seal-status in
		// addition to SectorIDA
		sectorIDB, _, errB := hB.AddPiece(context.Background(), RequireRandomBytes(t, hB.MaxBytesPerSector-50))
		require.NoError(t, errB)
		sectorIDSet.Store(sectorIDB, true)

		// seal everything
		hB.SectorBuilder.SealAllStagedSectors(context.Background())

		timeout := time.After(MaxTimeToSealASector * 2)
	Loop:
		for {
			select {
			case val := <-hB.SectorBuilder.SectorSealResults():
				require.NoError(t, val.SealingErr)
				sectorIDSet.Delete(val.SectorID)

				allHaveBeenSealed := true

				sectorIDSet.Range(func(key, value interface{}) bool {
					allHaveBeenSealed = false
					return false
				})

				if allHaveBeenSealed {
					break Loop
				}
			case <-timeout:
				t.Fatalf("timed out waiting for seal to complete")
			}
		}

		sectorIDSet.Range(func(sectorID, _ interface{}) bool {
			t.Fatalf("expected to have sealed everything, but still waiting on %d", sectorID)
			return false
		})
	})

	t.Run("proof-of-spacetime generation and verification", func(t *testing.T) {
		h := NewBuilder(t).Build()
		defer h.Close()

		inputBytes := RequireRandomBytes(t, h.MaxBytesPerSector)
		info, err := h.CreatePieceInfo(inputBytes)
		require.NoError(t, err)

		sectorID, err := h.SectorBuilder.AddPiece(context.Background(), info)
		require.NoError(t, err)

		timeout := time.After(MaxTimeToSealASector + MaxTimeToGenerateSectorPoSt)

		select {
		case val := <-h.SectorBuilder.SectorSealResults():
			require.NoError(t, val.SealingErr)
			require.Equal(t, sectorID, val.SealingResult.SectorID)

			// TODO: This should be generates from some standard source of
			// entropy, e.g. the blockchain
			challengeSeed := proofs.PoStChallengeSeed{1, 2, 3}

			// generate a proof-of-spacetime
			gres, gerr := h.SectorBuilder.GeneratePoSt(sectorbuilder.GeneratePoStRequest{
				CommRs:        []proofs.CommR{val.SealingResult.CommR},
				ChallengeSeed: challengeSeed,
			})
			require.NoError(t, gerr)

			// verify the proof-of-spacetime
			vres, verr := (&proofs.RustVerifier{}).VerifyPoST(proofs.VerifyPoSTRequest{
				ChallengeSeed: challengeSeed,
				CommRs:        []proofs.CommR{val.SealingResult.CommR},
				Faults:        gres.Faults,
				Proofs:        gres.Proofs,
				StoreType:     proofs.Test,
			})

			require.NoError(t, verr)
			require.True(t, vres.IsValid)
		case <-timeout:
			t.Fatalf("timed out waiting for seal to complete")
		}
	})
}
