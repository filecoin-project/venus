package sectorbuilder

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/stretchr/testify/require"
)

func TestSectorBuilder(t *testing.T) {
	t.Run("concurrent AddPiece and SealAllStagedSectors", func(t *testing.T) {
		t.Parallel()

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		// stringify the content identifiers to make them easily sortable later
		sealedPieceCidCh := make(chan string)
		addedPieceCidCh := make(chan string)
		errs := make(chan error)

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults() {
				switch val := raw.(type) {
				case error:
					errs <- val
				case *SealedSector:
					for _, pieceInfo := range val.pieces {
						sealedPieceCidCh <- pieceInfo.Ref.String()
					}
				}
			}
		}()

		autoSealsToSchedule := 5
		for i := 0; i < autoSealsToSchedule; i++ {
			go func(n int) {
				time.Sleep(time.Second * time.Duration(n))
				h.sectorBuilder.SealAllStagedSectors(context.Background())
			}(i)
		}

		piecesToSeal := 10
		for i := 0; i < piecesToSeal; i++ {
			go func() {
				pieceCid := h.requireAddPiece(requireRandomBytes(t, h.maxBytesPerSector/3))
				addedPieceCidCh <- pieceCid.String()
			}()
		}

		var addedPieceCids []string
		var sealedPieceCids []string

		// wait for a bit of time for the various seal() ops to complete and
		// capture the CIDs of added pieces for comparison with the CIDS of
		// sealed pieces
		timeout := time.After(120 * time.Second)
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

		// wait around for a few more seconds to ensure that there weren't any
		// superfluous seal() calls lingering
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

	t.Run("concurrent writes where size(piece) == max", func(t *testing.T) {
		t.Parallel()

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		// CIDs will be added to this map when given to the SectorBuilder and
		// removed when the CID has been sealed into a sector.
		pieceCidSet := sync.Map{}

		done := make(chan *cid.Cid)
		errs := make(chan error)

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults() {
				switch val := raw.(type) {
				case error:
					errs <- val
				case *SealedSector:
					for _, pieceInfo := range val.pieces {
						done <- pieceInfo.Ref
					}
				}
			}
		}()

		piecesToSeal := 10
		for i := 0; i < piecesToSeal; i++ {
			go func() {
				pieceCid := h.requireAddPiece(requireRandomBytes(t, h.maxBytesPerSector))
				pieceCidSet.Store(pieceCid, true)
			}()
		}

		// realistically, this should take 15-20 seconds
		timeout := time.After(120 * time.Second)
		for {
			if piecesToSeal == 0 {
				break
			}
			select {
			case err := <-errs:
				require.NoError(t, err)
			case sealed := <-done:
				pieceCidSet.Delete(sealed)
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

	t.Run("adding one piece causes two sectors to be sealed", func(t *testing.T) {
		t.Parallel()

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		// sector ids will be added to this map as we add pieces
		sectorIDSet := sync.Map{}

		// receives ids of sealed sectors
		done := make(chan uint64)
		errs := make(chan error)

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults() {
				switch val := raw.(type) {
				case error:
					errs <- val
				case *SealedSector:
					done <- val.SectorID
				}
			}
		}()

		// Add a piece with size(piece) < max. Then, add a piece with
		// size(piece) == max. The SectorBuilder should seal the first piece
		// into an unsealed sector (because the second one won't fit) and the
		// second piece too (because it completely fills the newly-provisioned
		// unsealed sector).

		// add a piece and grab the id of the sector to which it was added
		pieceInfoA := h.requirePieceInfo(requireRandomBytes(t, h.maxBytesPerSector-10))
		sectorIDA, err := h.sectorBuilder.AddPiece(h.ctx, pieceInfoA)
		require.NoError(t, err)
		sectorIDSet.Store(sectorIDA, true)

		// same thing with the second one
		pieceInfoB := h.requirePieceInfo(requireRandomBytes(t, h.maxBytesPerSector))
		sectorIDB, err := h.sectorBuilder.AddPiece(h.ctx, pieceInfoB)
		require.NoError(t, err)
		sectorIDSet.Store(sectorIDB, true)

		numSectorsToSeal := 2

		timeout := time.After(120 * time.Second)
		for {
			if numSectorsToSeal == 0 {
				break
			}
			select {
			case err := <-errs:
				require.NoError(t, err)
			case sealed := <-done:
				sectorIDSet.Delete(sealed)
				numSectorsToSeal--
			case <-timeout:
				t.Fatalf("timed out waiting for seal ops to complete (%d remaining)", numSectorsToSeal)
			}
		}

		sectorIDSet.Range(func(key, value interface{}) bool {
			t.Fatalf("should have removed each sector id from set as they were sealed (found %s)", key)
			return false
		})
	})
}
