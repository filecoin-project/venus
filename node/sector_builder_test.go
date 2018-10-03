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

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/proofs"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nodeWithSectorBuilder(t *testing.T) (*Node, *SectorBuilder, address.Address, uint64) {
	t.Helper()
	require := require.New(t)
	ctx := context.Background()

	nd := MakeOfflineNode(t)

	owner, err := nd.NewAddress()
	require.NoError(err)

	defaultAddr, err := nd.DefaultSenderAddress()
	require.NoError(err)

	tif := consensus.MakeGenesisFunc(
		consensus.ActorAccount(owner, types.NewAttoFILFromFIL(1000000)),
		consensus.ActorAccount(defaultAddr, types.NewAttoFILFromFIL(1000000)),
	)

	requireResetNodeGen(require, nd, tif)

	require.NoError(nd.Start(ctx))

	pledge := uint64(100)
	coll := *types.NewAttoFILFromFIL(100)
	result := <-RunCreateMiner(t, nd, owner, pledge, th.RequireRandomPeerID(), coll)
	require.NoError(result.Err)
	require.NotNil(result.MinerAddress)

	sstore := proofs.NewProofTestSectorStore(nd.Repo.(*repo.MemRepo).StagingDir(), nd.Repo.(*repo.MemRepo).SealedDir())

	res, err := sstore.GetMaxUnsealedBytesPerSector()
	require.NoError(err)

	sb, err := InitSectorBuilder(context.Background(), nd.Repo.Datastore(), nd.Blockservice, *result.MinerAddress, sstore, 0)
	require.NoError(err)

	return nd, sb, *result.MinerAddress, res.NumBytes
}

func requireAddPiece(ctx context.Context, t *testing.T, nd *Node, sb *SectorBuilder, pieceData []byte) *cid.Cid {
	pieceInfo := requirePieceInfo(t, nd, sb, pieceData)
	_, err := sb.AddPiece(ctx, pieceInfo)
	require.NoError(t, err)
	return pieceInfo.Ref
}

func createPieceInfo(nd *Node, sb *SectorBuilder, bytes []byte) (*PieceInfo, error) {
	data := dag.NewRawNode(bytes)

	if err := nd.Blockservice.AddBlock(data); err != nil {
		return nil, err
	}

	return sb.NewPieceInfo(data.Cid(), uint64(len(bytes)))
}

func requirePieceInfo(t *testing.T, nd *Node, sb *SectorBuilder, bytes []byte) *PieceInfo {
	info, err := createPieceInfo(nd, sb, bytes)
	require.NoError(t, err)
	return info
}

func TestSectorBuilder(t *testing.T) {
	t.Run("concurrent AddPiece and SealAllStagedSectors", func(t *testing.T) {
		t.Parallel()

		nd, sb, _, max := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		// stringify the content identifiers to make them easily sortable later
		sealedPieceCidCh := make(chan string)
		addedPieceCidCh := make(chan string)
		errs := make(chan error)

		go func() {
			for raw := range sb.SectorSealResults {
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
				sb.SealAllStagedSectors(context.Background())
			}(i)
		}

		piecesToSeal := 10
		for i := 0; i < piecesToSeal; i++ {
			go func() {
				pieceCid := requireAddPiece(context.Background(), t, nd, sb, requireRandomBytes(t, max/3))
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

		ctx := context.Background()

		nd, sb, _, max := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		// CIDs will be added to this map when given to the SectorBuilder and
		// removed when the CID has been sealed into a sector.
		pieceCidSet := sync.Map{}

		done := make(chan *cid.Cid)
		errs := make(chan error)

		go func() {
			for raw := range sb.SectorSealResults {
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
				pieceCid := requireAddPiece(ctx, t, nd, sb, requireRandomBytes(t, max))
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

		ctx := context.Background()

		nd, sb, _, max := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		// sector ids will be added to this map as we add pieces
		sectorIDSet := sync.Map{}

		// receives ids of sealed sectors
		done := make(chan uint64)
		errs := make(chan error)

		go func() {
			for raw := range sb.SectorSealResults {
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
		pieceInfoA := requirePieceInfo(t, nd, sb, requireRandomBytes(t, max-10))
		sectorIDA, err := sb.AddPiece(ctx, pieceInfoA)
		require.NoError(t, err)
		sectorIDSet.Store(sectorIDA, true)

		// same thing with the second one
		pieceInfoB := requirePieceInfo(t, nd, sb, requireRandomBytes(t, max))
		sectorIDB, err := sb.AddPiece(ctx, pieceInfoB)
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
		ctx := context.Background()

		nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		sector := sb.curUnsealedSector

		go func() {
			for raw := range sb.SectorSealResults {
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

		metadataMustMatch(require, sb, sb.curUnsealedSector, 0)

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

		metadataMustMatch(require, sb, sb.curUnsealedSector, 0)
		bytes1 := requireRandomBytes(t, 52) // len(text) = 52
		cid1 := requireAddPiece(ctx, t, nd, sb, bytes1)
		assert.Equal(sector, sb.curUnsealedSector)

		metadataMustMatch(require, sb, sector, 1)
		assert.Nil(sector.sealed)

		bytes2 := requireRandomBytes(t, 56)
		cid2 := requireAddPiece(ctx, t, nd, sb, bytes2)
		assert.Equal(sector, sb.curUnsealedSector)
		assert.Nil(sector.sealed)

		// persisted and calculated metadata match.
		metadataMustMatch(require, sb, sector, 2)

		// triggers seal, as piece won't fit
		bytes3 := requireRandomBytes(t, 58)
		requireAddPiece(ctx, t, nd, sb, bytes3)

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
		reader2, err := sb.ReadPieceFromSealedSector(cid2)
		require.NoError(err)
		cid2Bytes, err := ioutil.ReadAll(reader2)
		require.NoError(err)
		assert.True(bytes.Equal(bytes2, cid2Bytes))

		// persisted and calculated metadata match after a sector is sealed.
		metadataMustMatch(require, sb, sector, 2)

		newSector := sb.curUnsealedSector
		metadataMustMatch(require, sb, newSector, 1)

		sealed := sector.sealed
		assert.NotNil(sealed)
		assert.Nil(newSector.sealed)

		assert.Equal(sealed.unsealedSectorAccess, sector.unsealedSectorAccess)
		assert.Equal(sealed.pieces, sector.pieces)
		assert.Equal(sealed.numBytes, sector.numBytesUsed)

		meta := sb.curUnsealedSector.SectorMetadata()
		assert.Len(meta.Pieces, 1)
		assert.Equal(int(testSectorSize), int(meta.MaxBytes))
		assert.Equal(len(bytes3), int(meta.NumBytesUsed))

		_, err = createPieceInfo(nd, sb, requireRandomBytes(t, testSectorSize+10))
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

		ctx := context.Background()

		nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		firstSectorID := sb.curUnsealedSector.sectorID

		require.Equal(0, len(sb.curUnsealedSector.pieces))
		require.Equal(0, len(sb.sealedSectors))

		pieceCid := requireAddPiece(ctx, t, nd, sb, requireRandomBytes(t, testSectorSize-10))

		require.Equal(1, len(sb.curUnsealedSector.pieces))
		require.True(pieceCid.Equals(sb.curUnsealedSector.pieces[0].Ref))

		var sealingErr error
		sealingWg := sync.WaitGroup{}
		sealingWg.Add(1)

		go func() {
			for raw := range sb.SectorSealResults {
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

		sb.SealAllStagedSectors(ctx)

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(0, len(sb.curUnsealedSector.pieces))
		require.Equal(1, len(sb.sealedSectors))
		require.Equal(firstSectorID, sb.sealedSectors[0].sectorID)
	})

	t.Run("sealing sector moves metadata", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		ctx := context.Background()

		nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		a := testSectorSize / 2
		b := testSectorSize - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		_, err := io.ReadFull(rand.Reader, bytesA)
		require.NoError(err)

		_, err = io.ReadFull(rand.Reader, bytesB)
		require.NoError(err)

		sector := sb.curUnsealedSector

		go func() {
			for raw := range sb.SectorSealResults {
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

		requireAddPiece(ctx, t, nd, sb, bytesA)
		sb.AddPiece(ctx, requirePieceInfo(t, nd, sb, bytesA))
		sectormeta, err := sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.NoError(err)
		require.NotNil(sectormeta)

		sb.AddPiece(ctx, requirePieceInfo(t, nd, sb, bytesB))

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		_, err = sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.Error(err)
		require.Contains(err.Error(), "not found")

		sealedmeta, err := sb.metadataStore.getSealedSectorMetadata(sector.sealed.commR)
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

		ctx := context.Background()

		nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		sector := sb.curUnsealedSector

		bytesA := make([]byte, 10+(testSectorSize/2))

		sb.AddPiece(ctx, requirePieceInfo(t, nd, sb, bytesA))

		loaded, err := sb.metadataStore.getSector(sector.unsealedSectorAccess)
		require.NoError(err)

		sectorsMustEqual(t, sector, loaded)
	})

	t.Run("it loads a persisted, sealed sector", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		ctx := context.Background()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		a := testSectorSize / 2
		b := testSectorSize - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

		sector := sb.curUnsealedSector

		go func() {
			for raw := range sb.SectorSealResults {
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

		sb.AddPiece(ctx, requirePieceInfo(t, nd, sb, bytesA))
		sb.AddPiece(ctx, requirePieceInfo(t, nd, sb, bytesB))

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(1, len(sb.sealedSectors))
		sealedSector := sb.sealedSectors[0]

		loaded, err := sb.metadataStore.getSealedSector(sealedSector.commR)
		require.NoError(err)
		sealedSectorsMustEqual(t, sealedSector, loaded)
	})

	//////

	t.Run("it initializes a SectorBuilder from persisted metadata", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		ctx := context.Background()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		nd, sbA, minerAddr, testSectorSize := nodeWithSectorBuilder(t)
		defer nd.Stop(context.Background())

		a := testSectorSize / 2
		b := testSectorSize - a

		bytesA := make([]byte, a)
		bytesB := make([]byte, b)

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

		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, bytesA))

		// sector builder B should have the same state as sector builder A
		sstore := proofs.NewProofTestSectorStore(nd.Repo.(*repo.MemRepo).StagingDir(), nd.Repo.(*repo.MemRepo).SealedDir())

		sbB, err := InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, minerAddr, sstore, 0)
		require.NoError(err)

		// can't compare sectors with Equal(s1, s2) because their "file" fields will differ
		sectorBuildersMustEqual(t, sbA, sbB)

		// trigger sealing by adding a second piece
		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, bytesB))

		// wait for sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		// sector builder C should have the same state as sector builder A
		sbC, err := InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, minerAddr, sstore, 0)
		require.NoError(err)

		sectorBuildersMustEqual(t, sbA, sbC)

		// can't swap sector stores if their sector sizes differ
		sstore2 := proofs.NewDiskBackedSectorStore(nd.Repo.(*repo.MemRepo).StagingDir(), nd.Repo.(*repo.MemRepo).SealedDir())
		_, err = InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, minerAddr, sstore2, 0)
		require.Error(err)
	})

	t.Run("it truncates the file if file size > metadata size", func(t *testing.T) {
		t.Parallel()

		require := require.New(t)

		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, false, true)[0]
		defer nd.Stop(context.Background())

		nd.NewAddress() // TODO: default init make an address
		addr, err := nd.DefaultSenderAddress()
		require.NoError(err)

		sstore := proofs.NewProofTestSectorStore(nd.Repo.(*repo.MemRepo).StagingDir(), nd.Repo.(*repo.MemRepo).SealedDir())

		sbA, err := InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, addr, sstore, 0)
		require.NoError(err)

		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, make([]byte, 10)))
		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, make([]byte, 20)))
		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, make([]byte, 50)))

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
		sbB, err := InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, addr, sstore, 0)
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

		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, false, true)[0]
		defer nd.Stop(context.Background())

		nd.NewAddress() // TODO: default init make an address
		addr, err := nd.DefaultSenderAddress()
		require.NoError(err)

		sstore := proofs.NewProofTestSectorStore(nd.Repo.(*repo.MemRepo).StagingDir(), nd.Repo.(*repo.MemRepo).SealedDir())

		sbA, err := InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, addr, sstore, 0)
		require.NoError(err)

		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, make([]byte, 10)))
		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, make([]byte, 20)))
		sbA.AddPiece(ctx, requirePieceInfo(t, nd, sbA, make([]byte, 50)))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		// truncate the file such that its size < sum(size-of-pieces)
		require.NoError(os.Truncate(metaA.UnsealedSectorAccess, int64(40)))

		// initialize final sector builder
		sbB, err := InitSectorBuilder(nd.miningCtx, nd.Repo.Datastore(), nd.Blockservice, addr, sstore, 0)
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
