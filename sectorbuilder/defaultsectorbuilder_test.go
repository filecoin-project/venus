package sectorbuilder

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/filecoin-project/go-filecoin/proofs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSectorBuilder(t *testing.T) {
	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		assert := assert.New(t)
		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		sb := h.sectorBuilder.(*defaultSectorBuilder)
		sector := sb.curUnsealedSector
		var sealed *SealedSector

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults() {
				if raw.SealingErr != nil {
					sealingErr = raw.SealingErr
					sealingWg.Done()
				} else if raw.SealingResult != nil {
					if raw.SealingResult.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealed = raw.SealingResult
						sealingWg.Done()
					}
				}
			}
		}()

		metadataMustMatch(require, sb, sb.curUnsealedSector, nil, 0)

		// New unsealedSectorAccess is in the right places.
		stagingRes1, err := sb.sectorStore.NewStagingSectorAccess()
		require.NoError(err)

		sealedRes1, err := sb.sectorStore.NewSealedSectorAccess()
		require.NoError(err)

		// New unsealedSectorAccess is generated each time.
		stagingRes2, err := sb.sectorStore.NewStagingSectorAccess()
		require.NoError(err)

		sealedRes2, err := sb.sectorStore.NewSealedSectorAccess()
		require.NoError(err)

		assert.NotEqual(stagingRes1.SectorAccess, stagingRes2.SectorAccess)
		assert.NotEqual(sealedRes1.SectorAccess, sealedRes2.SectorAccess)

		metadataMustMatch(require, sb, sb.curUnsealedSector, nil, 0)
		bytes1 := requireRandomBytes(t, 52) // len(text) = 52
		cid1 := h.requireAddPiece(bytes1)
		assert.Equal(sector, sb.curUnsealedSector)

		metadataMustMatch(require, sb, sector, nil, 1)

		bytes2 := requireRandomBytes(t, 56)
		cid2 := h.requireAddPiece(bytes2)
		assert.Equal(sector, sb.curUnsealedSector)

		// persisted and calculated metadata match.
		metadataMustMatch(require, sb, sector, nil, 2)

		// triggers seal, as piece won't fit
		bytes3 := requireRandomBytes(t, 58)
		h.requireAddPiece(bytes3)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		assert.NotEqual(sector, sb.curUnsealedSector)

		// unseal first piece and confirm its bytes
		reader1, err := sb.ReadPieceFromSealedSector(cid1)
		require.NoError(err)
		cid1Bytes, err := ioutil.ReadAll(reader1)
		require.NoError(err)
		assert.True(bytes.Equal(bytes1, cid1Bytes))

		// unseal second piece and confirm its bytes, too
		reader2, err := h.sectorBuilder.ReadPieceFromSealedSector(cid2)
		require.NoError(err)
		cid2Bytes, err := ioutil.ReadAll(reader2)
		require.NoError(err)
		assert.True(bytes.Equal(bytes2, cid2Bytes))

		// persisted and calculated metadata match after a sector is sealed.
		metadataMustMatch(require, sb, sector, sealed, 2)

		newSector := sb.curUnsealedSector
		metadataMustMatch(require, sb, newSector, nil, 1)

		assert.NotNil(sealed)

		assert.Equal(sealed.unsealedSectorAccess, sector.unsealedSectorAccess)
		assert.Equal(sealed.pieces, sector.pieces)
		assert.Equal(sealed.numBytes, sector.numBytesUsed)

		meta := sb.curUnsealedSector.dumpCurrentState()
		assert.Len(meta.Pieces, 1)
		assert.Equal(int(sb.MaxBytesPerSector()), int(meta.MaxBytes))
		assert.Equal(len(bytes3), int(meta.NumBytesUsed))

		_, err = h.addPiece(requireRandomBytes(t, h.maxBytesPerSector+10))
		assert.EqualError(err, ErrPieceTooLarge.Error())
	})

	t.Run("can seal manually", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		sb := h.sectorBuilder.(*defaultSectorBuilder)

		firstSectorID := sb.curUnsealedSector.SectorID

		require.Equal(0, len(sb.curUnsealedSector.pieces))
		require.Equal(0, len(sb.sealedSectors))

		pieceCid := h.requireAddPiece(requireRandomBytes(t, h.maxBytesPerSector-10))

		require.Equal(1, len(sb.curUnsealedSector.pieces))
		require.True(pieceCid.Equals(sb.curUnsealedSector.pieces[0].Ref))

		var sealingErr error
		sealingWg := sync.WaitGroup{}
		sealingWg.Add(1)

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults() {
				if raw.SealingErr != nil {
					sealingErr = raw.SealingErr
					sealingWg.Done()
				} else if raw.SealingResult != nil {
					if raw.SealingResult.SectorID == firstSectorID {
						sealingWg.Done()
					}
				}
			}
		}()

		sb.SealAllStagedSectors(h.ctx)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(0, len(sb.curUnsealedSector.pieces))
		require.Equal(1, len(sb.sealedSectors))
		require.Equal(firstSectorID, sb.sealedSectors[0].SectorID)
	})

	t.Run("sealing sector moves metadata", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		sb := h.sectorBuilder.(*defaultSectorBuilder)

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		a := sb.MaxBytesPerSector() / 2
		b := sb.MaxBytesPerSector() - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		_, err := io.ReadFull(rand.Reader, bytesA)
		require.NoError(err)

		_, err = io.ReadFull(rand.Reader, bytesB)
		require.NoError(err)

		sector := sb.curUnsealedSector
		var sealed *SealedSector

		go func() {
			for raw := range sb.SectorSealResults() {
				if raw.SealingErr != nil {
					sealingErr = raw.SealingErr
					sealingWg.Done()
				} else if raw.SealingResult != nil {
					if raw.SealingResult.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealed = raw.SealingResult
						sealingWg.Done()
					}
				}
			}
		}()

		h.requireAddPiece(bytesA)
		h.requireAddPiece(bytesA)
		sectormeta, err := sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.NoError(err)
		require.NotNil(sectormeta)

		h.requireAddPiece(bytesB)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		_, err = sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.Error(err)
		require.Contains(err.Error(), "not found")

		sealedmeta, err := sb.metadataStore.getSealedSectorMetadata(sealed.CommR)
		require.NoError(err)
		require.NotNil(sealedmeta)

		require.Equal(sector.numBytesUsed, sealedmeta.NumBytes)
		require.Equal(len(sector.pieces), len(sealedmeta.Pieces))
		for i := 0; i < len(sector.pieces); i++ {
			pieceInfoMustEqual(t, sector.pieces[i], sealedmeta.Pieces[i])
		}
	})

	t.Run("it loads a persisted sector", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		sb := h.sectorBuilder.(*defaultSectorBuilder)

		sector := sb.curUnsealedSector

		bytesA := make([]byte, 10+(sb.MaxBytesPerSector()/2))

		h.requireAddPiece(bytesA)

		loaded, err := sb.metadataStore.getSector(sector.unsealedSectorAccess)
		require.NoError(err)

		sectorsMustEqual(t, sector, loaded)
	})

	t.Run("it loads a persisted, sealed sector", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		sb := h.sectorBuilder.(*defaultSectorBuilder)

		a := h.maxBytesPerSector / 2
		b := h.maxBytesPerSector - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		sector := sb.curUnsealedSector

		go func() {
			for raw := range sb.SectorSealResults() {
				if raw.SealingErr != nil {
					sealingErr = raw.SealingErr
					sealingWg.Done()
				} else if raw.SealingResult != nil {
					if raw.SealingResult != nil && raw.SealingResult.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealingWg.Done()
					}
				}
			}
		}()

		h.requireAddPiece(bytesA)
		h.requireAddPiece(bytesB)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(1, len(sb.sealedSectors))
		sealedSector := sb.sealedSectors[0]

		loaded, err := sb.metadataStore.getSealedSector(sealedSector.CommR)
		require.NoError(err)
		sealedSectorsMustEqual(t, sealedSector, loaded)
	})

	t.Run("it initializes a SectorBuilder from persisted metadata", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		a := h.maxBytesPerSector / 2
		b := h.maxBytesPerSector - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		sbA := h.sectorBuilder.(*defaultSectorBuilder)

		sector := sbA.curUnsealedSector

		go func() {
			for raw := range sbA.SectorSealResults() {
				if raw.SealingErr != nil {
					sealingErr = raw.SealingErr
					sealingWg.Done()
				} else if raw.SealingResult != nil {
					if raw.SealingResult.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealingWg.Done()
					}
				}
			}
		}()

		h.requireAddPiece(bytesA)

		// sector builder B should have the same state as sector builder A
		sstore := proofs.NewProofTestSectorStore(h.repo.StagingDir(), h.repo.SealedDir())

		sbB, err := Init(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		// can't compare sectors with Equal(s1, s2) because their "file" fields will differ
		sectorBuildersMustEqual(t, sbA, sbB.(*defaultSectorBuilder))

		// trigger sealing by adding a second piece
		h.requireAddPiece(bytesB)

		// wait for sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		// sector builder C should have the same state as sector builder A
		sbC, err := Init(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		sectorBuildersMustEqual(t, sbA, sbC.(*defaultSectorBuilder))

		// can't swap sector stores if their sector sizes differ
		sstore2 := proofs.NewDiskBackedSectorStore(h.repo.StagingDir(), h.repo.SealedDir())
		_, err = Init(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore2, 0)
		require.Error(err)
	})

	t.Run("it truncates the file if file size > metadata size", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		sbA := h.sectorBuilder.(*defaultSectorBuilder)

		h.requireAddPiece(requireRandomBytes(t, 10))
		h.requireAddPiece(requireRandomBytes(t, 20))
		h.requireAddPiece(requireRandomBytes(t, 50))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		// size of file on disk should match what we've persisted as metadata
		resA, errA := sbA.sectorStore.GetNumBytesUnsealed(proofs.GetNumBytesUnsealedRequest{
			SectorAccess: metaA.UnsealedSectorAccess,
		})
		require.NoError(errA)
		require.Equal(int(metaA.NumBytesUsed), int(resA.NumBytes))

		// perform an out-of-band write to the file (replaces its contents)
		ioutil.WriteFile(metaA.UnsealedSectorAccess, make([]byte, 90), 0600)

		// initialize a new sector builder (simulates the node restarting)
		sstore := proofs.NewProofTestSectorStore(h.repo.StagingDir(), h.repo.SealedDir())
		sbB, err := Init(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		metaB, err := sbB.(*defaultSectorBuilder).metadataStore.getSectorMetadata(sbB.(*defaultSectorBuilder).curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		// ensure that the file was truncated to match metadata
		resB, errB := sbA.sectorStore.GetNumBytesUnsealed(proofs.GetNumBytesUnsealedRequest{
			SectorAccess: metaB.UnsealedSectorAccess,
		})
		require.NoError(errB)
		require.Equal(int(resA.NumBytes), int(resB.NumBytes))
	})

	t.Run("it truncates the metadata if file size < metadata size", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t, golang)
		defer h.close()

		sbA := h.sectorBuilder.(*defaultSectorBuilder)

		h.requireAddPiece(requireRandomBytes(t, 10))
		h.requireAddPiece(requireRandomBytes(t, 20))
		h.requireAddPiece(requireRandomBytes(t, 50))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		// truncate the file such that its size < sum(size-of-pieces)
		require.NoError(os.Truncate(metaA.UnsealedSectorAccess, int64(40)))

		// initialize final sector builder
		sstore := proofs.NewProofTestSectorStore(h.repo.StagingDir(), h.repo.SealedDir())
		sbB, err := Init(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		metaB, err := sbA.metadataStore.getSectorMetadata(sbB.(*defaultSectorBuilder).curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		resB, errB := sbA.sectorStore.GetNumBytesUnsealed(proofs.GetNumBytesUnsealedRequest{
			SectorAccess: metaB.UnsealedSectorAccess,
		})
		require.NoError(errB)

		// ensure metadata was truncated
		require.Equal(2, len(metaB.Pieces))
		require.Equal(30, int(metaB.NumBytesUsed))

		// ensure that the file was truncated to match metadata
		require.Equal(int(metaB.NumBytesUsed), int(resB.NumBytes))
	})
}
