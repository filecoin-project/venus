package sectorbuilder

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"sort"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/stretchr/testify/require"
)

func TestSectorBuilder(t *testing.T) {
	t.Run("concurrent AddPiece and SealAllStagedSectors", func(t *testing.T) {
		t.Parallel()

		for _, cfg := range []sectorBuilderType{golang, rust} {
			func() {
				h := newSectorBuilderTestHarness(context.Background(), t, cfg)
				defer h.close()

				// stringify the content identifiers to make them easily
				// sortable later
				sealedPieceCidCh := make(chan string)
				addedPieceCidCh := make(chan string)
				errs := make(chan error)

				go func() {
					for val := range h.sectorBuilder.SectorSealResults() {
						if val.SealingErr != nil {
							errs <- val.SealingErr
						} else if val.SealingResult != nil {
							for _, pieceInfo := range val.SealingResult.pieces {
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

				// wait for a bit of time for the various seal() ops to complete
				// and capture the CIDs of added pieces for comparison with the
				// CIDS of sealed pieces
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
			}()
		}
	})

	t.Run("concurrent writes", func(t *testing.T) {
		t.Parallel()

		for _, cfg := range []sectorBuilderType{golang, rust} {
			func() {
				h := newSectorBuilderTestHarness(context.Background(), t, cfg)
				defer h.close()

				// CIDs will be added to this map when given to the SectorBuilder and
				// removed when the CID has been sealed into a sector.
				pieceCidSet := sync.Map{}

				done := make(chan *cid.Cid)
				errs := make(chan error)

				go func() {
					for val := range h.sectorBuilder.SectorSealResults() {
						if val.SealingErr != nil {
							errs <- val.SealingErr
						} else if val.SealingResult != nil {
							for _, pieceInfo := range val.SealingResult.pieces {
								done <- pieceInfo.Ref
							}
						}
					}
				}()

				piecesToSeal := 5
				for i := 0; i < piecesToSeal; i++ {
					go func() {
						pieceCid := h.requireAddPiece(requireRandomBytes(t, h.maxBytesPerSector))
						pieceCidSet.Store(pieceCid.String(), true)
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
						pieceCidSet.Delete(sealed.String())
						piecesToSeal--
					case <-timeout:
						t.Fatalf("timed out waiting for seal ops to complete (%d remaining)", piecesToSeal)
					}
				}

				// make some basic assertions about the output of
				// SectorBuilder#SealedSectors()
				sealedSectors, err := h.sectorBuilder.SealedSectors()
				require.NoError(t, err)
				require.Equal(t, len(sealedSectors), 5)
				for _, meta := range sealedSectors {
					require.NotEqual(t, 0, len(meta.pieces))
				}

				pieceCidSet.Range(func(key, value interface{}) bool {
					t.Fatalf("should have removed each piece from set as they were sealed (found %s)", key)
					return false
				})
			}()
		}
	})

	t.Run("add, seal, read (by unsealing) user piece-bytes", func(t *testing.T) {
		t.Parallel()

		for _, cfg := range []sectorBuilderType{golang, rust} {
			func() {
				h := newSectorBuilderTestHarness(context.Background(), t, cfg)
				defer h.close()

				inputBytes := requireRandomBytes(t, h.maxBytesPerSector)
				info, err := h.createPieceInfo(inputBytes)
				require.NoError(t, err)

				sectorID, err := h.sectorBuilder.AddPiece(context.Background(), info)
				require.NoError(t, err)

				timeout := time.After(120 * time.Second)
			Loop:
				for {
					select {
					case val := <-h.sectorBuilder.SectorSealResults():
						require.NoError(t, val.SealingErr)
						require.Equal(t, sectorID, val.SealingResult.SectorID)
						break Loop
					case <-timeout:
						break Loop // I've always dreamt of using GOTO
					}
				}

				reader, err := h.sectorBuilder.ReadPieceFromSealedSector(info.Ref)
				require.NoError(t, err)

				outputBytes, err := ioutil.ReadAll(reader)
				require.NoError(t, err)

				require.Equal(t, hex.EncodeToString(inputBytes), hex.EncodeToString(outputBytes))
			}()
		}
	})

	t.Run("returns empty list of sealed sector metadata", func(t *testing.T) {
		t.Parallel()

		for _, cfg := range []sectorBuilderType{golang, rust} {
			func() {
				h := newSectorBuilderTestHarness(context.Background(), t, cfg)
				defer h.close()

				sealedSectors, err := h.sectorBuilder.SealedSectors()
				require.NoError(t, err)
				require.Equal(t, 0, len(sealedSectors))
			}()
		}
	})
}
