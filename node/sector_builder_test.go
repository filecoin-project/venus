package node

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	bserv "gx/ipfs/QmTfTKeBhTLjSjxXQsjkF2b1DfZmYEMnknGE2y2gX57C6v/go-blockservice"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	"gx/ipfs/QmZxjqR9Qgompju73kakSoUj3rbVndAzky3oCDiBNCxPs1/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sectorBuilderTestHarness struct {
	blockService  bserv.BlockService
	ctx           context.Context
	minerAddr     address.Address
	repo          repo.Repo
	sectorBuilder *SectorBuilder
	t             *testing.T
}

func newSectorBuilderTestHarness(ctx context.Context, t *testing.T) sectorBuilderTestHarness {
	memRepo := repo.NewInMemoryRepo()
	blockStore := bstore.NewBlockstore(memRepo.Datastore())
	blockService := bserv.New(blockStore, offline.Exchange(blockStore))
	sectorStore := proofs.NewProofTestSectorStore(memRepo.StagingDir(), memRepo.SealedDir())
	minerAddr := address.MakeTestAddress("wombat")

	sectorBuilder, err := InitSectorBuilder(ctx, memRepo.Datastore(), blockService, minerAddr, sectorStore, 0)
	require.NoError(t, err)

	return sectorBuilderTestHarness{
		ctx:           ctx,
		t:             t,
		repo:          memRepo,
		blockService:  blockService,
		sectorBuilder: sectorBuilder,
		minerAddr:     minerAddr,
	}
}

func (h sectorBuilderTestHarness) close() error {
	close(h.sectorBuilder.SectorSealResults)
	return h.repo.Close()
}

func (h sectorBuilderTestHarness) requirePieceInfo(pieceData []byte) *PieceInfo {
	pieceInfo, err := h.createPieceInfo(pieceData)
	require.NoError(h.t, err)

	return pieceInfo
}

func (h sectorBuilderTestHarness) requireAddPiece(pieceData []byte) *cid.Cid {
	pieceInfo, err := h.createPieceInfo(pieceData)
	require.NoError(h.t, err)

	_, err = h.sectorBuilder.AddPiece(h.ctx, pieceInfo)
	require.NoError(h.t, err)

	return pieceInfo.Ref
}

func (h sectorBuilderTestHarness) createPieceInfo(pieceData []byte) (*PieceInfo, error) {
	data := dag.NewRawNode(pieceData)

	if err := h.blockService.AddBlock(data); err != nil {
		return nil, err
	}

	return h.sectorBuilder.NewPieceInfo(data.Cid(), uint64(len(pieceData)))
}

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
			for raw := range h.sectorBuilder.SectorSealResults {
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
				pieceCid := h.requireAddPiece(requireRandomBytes(t, h.sectorBuilder.sectorSize/3))
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
			for raw := range h.sectorBuilder.SectorSealResults {
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
				pieceCid := h.requireAddPiece(requireRandomBytes(t, h.sectorBuilder.sectorSize))
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
			for raw := range h.sectorBuilder.SectorSealResults {
				switch val := raw.(type) {
				case error:
					errs <- val
				case *SealedSector:
					done <- val.sectorID
				}
			}
		}()

		// Add a piece with size(piece) < max. Then, add a piece with
		// size(piece) == max. The SectorBuilder should seal the first piece
		// into an unsealed sector (because the second one won't fit) and the
		// second piece too (because it completely fills the newly-provisioned
		// unsealed sector).

		// add a piece and grab the id of the sector to which it was added
		pieceInfoA := h.requirePieceInfo(requireRandomBytes(t, h.sectorBuilder.sectorSize-10))
		sectorIDA, err := h.sectorBuilder.AddPiece(h.ctx, pieceInfoA)
		require.NoError(t, err)
		sectorIDSet.Store(sectorIDA, true)

		// same thing with the second one
		pieceInfoB := h.requirePieceInfo(requireRandomBytes(t, h.sectorBuilder.sectorSize))
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

	t.Run("basic flow", func(t *testing.T) {
		t.Parallel()

		assert := assert.New(t)
		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		sector := h.sectorBuilder.curUnsealedSector

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults {
				switch val := raw.(type) {
				case error:
					sealingErr = val
					sealingWg.Done()
				case *SealedSector:
					if val != nil && val.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealingWg.Done()
					}
				}

				return
			}
		}()

		metadataMustMatch(require, h.sectorBuilder, h.sectorBuilder.curUnsealedSector, 0)

		// New unsealedSectorAccess is in the right places.
		stagingRes1, err := h.sectorBuilder.sectorStore.NewStagingSectorAccess()
		require.NoError(err)

		sealedRes1, err := h.sectorBuilder.sectorStore.NewSealedSectorAccess()
		require.NoError(err)

		// New unsealedSectorAccess is generated each time.
		stagingRes2, err := h.sectorBuilder.sectorStore.NewStagingSectorAccess()
		require.NoError(err)

		sealedRes2, err := h.sectorBuilder.sectorStore.NewSealedSectorAccess()
		require.NoError(err)

		assert.NotEqual(stagingRes1.SectorAccess, stagingRes2.SectorAccess)
		assert.NotEqual(sealedRes1.SectorAccess, sealedRes2.SectorAccess)

		metadataMustMatch(require, h.sectorBuilder, h.sectorBuilder.curUnsealedSector, 0)
		bytes1 := requireRandomBytes(t, 52) // len(text) = 52
		cid1 := h.requireAddPiece(bytes1)
		assert.Equal(sector, h.sectorBuilder.curUnsealedSector)

		metadataMustMatch(require, h.sectorBuilder, sector, 1)
		assert.Nil(sector.sealed)

		bytes2 := requireRandomBytes(t, 56)
		cid2 := h.requireAddPiece(bytes2)
		assert.Equal(sector, h.sectorBuilder.curUnsealedSector)
		assert.Nil(sector.sealed)

		// persisted and calculated metadata match.
		metadataMustMatch(require, h.sectorBuilder, sector, 2)

		// triggers seal, as piece won't fit
		bytes3 := requireRandomBytes(t, 58)
		h.requireAddPiece(bytes3)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		assert.NotEqual(sector, h.sectorBuilder.curUnsealedSector)

		// unseal first piece and confirm its bytes
		reader1, err := h.sectorBuilder.ReadPieceFromSealedSector(cid1)
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
		metadataMustMatch(require, h.sectorBuilder, sector, 2)

		newSector := h.sectorBuilder.curUnsealedSector
		metadataMustMatch(require, h.sectorBuilder, newSector, 1)

		sealed := sector.sealed
		assert.NotNil(sealed)
		assert.Nil(newSector.sealed)

		assert.Equal(sealed.unsealedSectorAccess, sector.unsealedSectorAccess)
		assert.Equal(sealed.pieces, sector.pieces)
		assert.Equal(sealed.numBytes, sector.numBytesUsed)

		meta := h.sectorBuilder.curUnsealedSector.SectorMetadata()
		assert.Len(meta.Pieces, 1)
		assert.Equal(int(h.sectorBuilder.sectorSize), int(meta.MaxBytes))
		assert.Equal(len(bytes3), int(meta.NumBytesUsed))

		_, err = h.createPieceInfo(requireRandomBytes(t, h.sectorBuilder.sectorSize+10))
		assert.EqualError(err, ErrPieceTooLarge.Error())
	})

	t.Run("creating datastore keys", func(t *testing.T) {
		t.Parallel()

		assert := assert.New(t)

		label := "SECTORFILENAMEWHATEVER"

		k := metadataKey(label).String()
		// Don't accidentally test Datastore namespacing implementation.
		assert.Contains(k, "sectors")
		assert.Contains(k, "metadata")
		assert.Contains(k, label)

		var merkleRoot [32]byte
		copy(merkleRoot[:], ([]byte)("someMerkleRootLOL")[0:32])

		k2 := sealedMetadataKey(merkleRoot).String()
		// Don't accidentally test Datastore namespacing implementation.
		assert.Contains(k2, "sealedSectors")
		assert.Contains(k2, "metadata")
		assert.Contains(k2, commRString(merkleRoot))
	})

	t.Run("can seal manually", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		firstSectorID := h.sectorBuilder.curUnsealedSector.sectorID

		require.Equal(0, len(h.sectorBuilder.curUnsealedSector.pieces))
		require.Equal(0, len(h.sectorBuilder.sealedSectors))

		pieceCid := h.requireAddPiece(requireRandomBytes(t, h.sectorBuilder.sectorSize-10))

		require.Equal(1, len(h.sectorBuilder.curUnsealedSector.pieces))
		require.True(pieceCid.Equals(h.sectorBuilder.curUnsealedSector.pieces[0].Ref))

		var sealingErr error
		sealingWg := sync.WaitGroup{}
		sealingWg.Add(1)

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults {
				switch val := raw.(type) {
				case error:
					sealingErr = val
					sealingWg.Done()
				case *SealedSector:
					if val != nil && val.sectorID == firstSectorID {
						sealingWg.Done()
					}
				}

				return
			}
		}()

		h.sectorBuilder.SealAllStagedSectors(h.ctx)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(0, len(h.sectorBuilder.curUnsealedSector.pieces))
		require.Equal(1, len(h.sectorBuilder.sealedSectors))
		require.Equal(firstSectorID, h.sectorBuilder.sealedSectors[0].sectorID)
	})

	t.Run("sealing sector moves metadata", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		a := h.sectorBuilder.sectorSize / 2
		b := h.sectorBuilder.sectorSize - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		_, err := io.ReadFull(rand.Reader, bytesA)
		require.NoError(err)

		_, err = io.ReadFull(rand.Reader, bytesB)
		require.NoError(err)

		sector := h.sectorBuilder.curUnsealedSector

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults {
				switch val := raw.(type) {
				case error:
					sealingErr = val
					sealingWg.Done()
				case *SealedSector:
					if val != nil && val.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealingWg.Done()
					}
				}

				return
			}
		}()

		h.requireAddPiece(bytesA)
		h.requireAddPiece(bytesA)
		sectormeta, err := h.sectorBuilder.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.NoError(err)
		require.NotNil(sectormeta)

		h.requireAddPiece(bytesB)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		_, err = h.sectorBuilder.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.Error(err)
		require.Contains(err.Error(), "not found")

		sealedmeta, err := h.sectorBuilder.metadataStore.getSealedSectorMetadata(sector.sealed.commR)
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

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		sector := h.sectorBuilder.curUnsealedSector

		bytesA := make([]byte, 10+(h.sectorBuilder.sectorSize/2))

		h.requireAddPiece(bytesA)

		loaded, err := h.sectorBuilder.metadataStore.getSector(sector.unsealedSectorAccess)
		require.NoError(err)

		sectorsMustEqual(t, sector, loaded)
	})

	t.Run("it loads a persisted, sealed sector", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		h := newSectorBuilderTestHarness(context.Background(), t)

		a := h.sectorBuilder.sectorSize / 2
		b := h.sectorBuilder.sectorSize - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		sector := h.sectorBuilder.curUnsealedSector

		go func() {
			for raw := range h.sectorBuilder.SectorSealResults {
				switch val := raw.(type) {
				case error:
					sealingErr = val
					sealingWg.Done()
				case *SealedSector:
					if val != nil && val.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealingWg.Done()
					}
				}

				return
			}
		}()

		h.requireAddPiece(bytesA)
		h.requireAddPiece(bytesB)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(1, len(h.sectorBuilder.sealedSectors))
		sealedSector := h.sectorBuilder.sealedSectors[0]

		loaded, err := h.sectorBuilder.metadataStore.getSealedSector(sealedSector.commR)
		require.NoError(err)
		sealedSectorsMustEqual(t, sealedSector, loaded)
	})

	t.Run("it initializes a SectorBuilder from persisted metadata", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		a := h.sectorBuilder.sectorSize / 2
		b := h.sectorBuilder.sectorSize - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		sbA := h.sectorBuilder

		sector := sbA.curUnsealedSector

		go func() {
			for raw := range sbA.SectorSealResults {
				switch val := raw.(type) {
				case error:
					sealingErr = val
					sealingWg.Done()
				case *SealedSector:
					if val != nil && val.unsealedSectorAccess == sector.unsealedSectorAccess {
						sealingWg.Done()
					}
				}

				return
			}
		}()

		h.requireAddPiece(bytesA)

		// sector builder B should have the same state as sector builder A
		sstore := proofs.NewProofTestSectorStore(h.repo.(*repo.MemRepo).StagingDir(), h.repo.(*repo.MemRepo).SealedDir())

		sbB, err := InitSectorBuilder(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		// can't compare sectors with Equal(s1, s2) because their "file" fields will differ
		sectorBuildersMustEqual(t, sbA, sbB)

		// trigger sealing by adding a second piece
		h.requireAddPiece(bytesB)

		// wait for sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		// sector builder C should have the same state as sector builder A
		sbC, err := InitSectorBuilder(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		sectorBuildersMustEqual(t, sbA, sbC)

		// can't swap sector stores if their sector sizes differ
		sstore2 := proofs.NewDiskBackedSectorStore(h.repo.(*repo.MemRepo).StagingDir(), h.repo.(*repo.MemRepo).SealedDir())
		_, err = InitSectorBuilder(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore2, 0)
		require.Error(err)
	})

	t.Run("it truncates the file if file size > metadata size", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		sbA := h.sectorBuilder

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
		sstore := proofs.NewProofTestSectorStore(h.repo.(*repo.MemRepo).StagingDir(), h.repo.(*repo.MemRepo).SealedDir())
		sbB, err := InitSectorBuilder(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		metaB, err := sbB.metadataStore.getSectorMetadata(sbB.curUnsealedSector.unsealedSectorAccess)
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

		h := newSectorBuilderTestHarness(context.Background(), t)
		defer h.close()

		sbA := h.sectorBuilder

		h.requireAddPiece(requireRandomBytes(t, 10))
		h.requireAddPiece(requireRandomBytes(t, 20))
		h.requireAddPiece(requireRandomBytes(t, 50))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		// truncate the file such that its size < sum(size-of-pieces)
		require.NoError(os.Truncate(metaA.UnsealedSectorAccess, int64(40)))

		// initialize final sector builder
		sstore := proofs.NewProofTestSectorStore(h.repo.(*repo.MemRepo).StagingDir(), h.repo.(*repo.MemRepo).SealedDir())
		sbB, err := InitSectorBuilder(h.ctx, h.repo.Datastore(), h.blockService, h.minerAddr, sstore, 0)
		require.NoError(err)

		metaB, err := sbA.metadataStore.getSectorMetadata(sbB.curUnsealedSector.unsealedSectorAccess)
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

	t.Run("prover id creation", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		hash := address.Hash([]byte("satoshi"))
		addr := address.NewMainnet(hash)

		id := addressToProverID(addr)

		require.Equal(31, len(id))
	})
}

func metadataMustMatch(require *require.Assertions, sb *SectorBuilder, sector *UnsealedSector, pieces int) {
	sealed := sector.sealed
	if sealed != nil {
		sealedMeta := sealed.SealedSectorMetadata()
		sealedMetaPersisted, err := sb.metadataStore.getSealedSectorMetadata(sealed.commR)
		require.NoError(err)
		require.Equal(sealedMeta, sealedMetaPersisted)
	} else {
		meta := sector.SectorMetadata()
		require.Len(meta.Pieces, pieces)

		// persisted and calculated metadata match.
		metaPersisted, err := sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.NoError(err)
		require.Equal(metaPersisted, meta)
	}

	builderMeta := sb.dumpCurrentState()
	builderMetaPersisted, err := sb.metadataStore.getSectorBuilderMetadata(sb.minerAddr)
	require.NoError(err)
	require.Equal(builderMeta, builderMetaPersisted)
}

func pieceInfoMustEqual(t *testing.T, p1 *PieceInfo, p2 *PieceInfo) {
	if p1.Size != p2.Size {
		t.Fatalf("p1.size(%d) != p2.size(%d)\n", p1.Size, p2.Size)
	}

	if !p1.Ref.Equals(p2.Ref) {
		t.Fatalf("p1.Ref(%s) != p2.Ref(%s)\n", p1.Ref.String(), p2.Ref.String())
	}
}

func sectorBuildersMustEqual(t *testing.T, sb1 *SectorBuilder, sb2 *SectorBuilder) {
	require := require.New(t)

	require.Equal(sb1.minerAddr, sb2.minerAddr)
	require.Equal(sb1.sectorSize, sb2.sectorSize)

	sectorsMustEqual(t, sb1.curUnsealedSector, sb2.curUnsealedSector)

	require.Equal(len(sb1.sealedSectors), len(sb2.sealedSectors))
	for i := 0; i < len(sb1.sealedSectors); i++ {
		sealedSectorsMustEqual(t, sb1.sealedSectors[i], sb2.sealedSectors[i])
	}
}

func sealedSectorsMustEqual(t *testing.T, ss1 *SealedSector, ss2 *SealedSector) {
	require := require.New(t)

	if ss1 == nil && ss2 == nil {
		return
	}

	require.Equal(ss1.sealedSectorAccess, ss2.sealedSectorAccess)
	require.Equal(ss1.unsealedSectorAccess, ss2.unsealedSectorAccess)
	require.Equal(ss1.numBytes, ss2.numBytes)
	require.True(bytes.Equal(ss1.commR[:], ss2.commR[:]))

	require.Equal(len(ss1.pieces), len(ss2.pieces))
	for i := 0; i < len(ss1.pieces); i++ {
		pieceInfoMustEqual(t, ss1.pieces[i], ss2.pieces[i])
	}
}

func sectorsMustEqual(t *testing.T, s1 *UnsealedSector, s2 *UnsealedSector) {
	require := require.New(t)

	require.Equal(s1.unsealedSectorAccess, s2.unsealedSectorAccess)
	require.Equal(s1.maxBytes, s2.maxBytes)
	require.Equal(s1.numBytesUsed, s2.numBytesUsed)

	sealedSectorsMustEqual(t, s1.sealed, s2.sealed)

	require.Equal(len(s1.pieces), len(s2.pieces))
	for i := 0; i < len(s1.pieces); i++ {
		pieceInfoMustEqual(t, s1.pieces[i], s2.pieces[i])
	}
}

func requireRandomBytes(t *testing.T, n uint64) []byte {
	slice := make([]byte, n)

	_, err := io.ReadFull(rand.Reader, slice)
	require.NoError(t, err)

	return slice
}
