package node

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/proofs"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tempSectorDirs struct {
	stagingPath string
	sealedPath  string
}

var _ SectorDirs = &tempSectorDirs{}

func newTempSectorDirs() *tempSectorDirs {
	return &tempSectorDirs{
		stagingPath: randTempDir(),
		sealedPath:  randTempDir(),
	}
}

func (f *tempSectorDirs) StagingDir() string {
	return f.stagingPath
}

func (f *tempSectorDirs) SealedDir() string {
	return f.sealedPath
}

func (f *tempSectorDirs) remove() {
	if err := os.RemoveAll(f.sealedPath); err != nil {
		panic(err)
	}

	if err := os.RemoveAll(f.stagingPath); err != nil {
		panic(err)
	}
}

// randTempDir creates a subdirectory of os.TempDir() with a
// randomized name and returns the directory's path
func randTempDir() string {
	// create a random string
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		panic("couldn't read")
	}
	encoded := base32.StdEncoding.EncodeToString(b)

	// append string to temp dir path
	path := filepath.Join(os.TempDir(), encoded)

	// create the directory
	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("couldn't create temp dir %s", path))
	}

	return path
}

func TestSectorBuilderSimple(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	dirs, _, sb, _, _ := nodeWithSectorBuilder(t)
	defer dirs.remove()

	sector, err := sb.NewSector()
	require.NoError(err)
	ctx := context.Background()

	d1Data := []byte("hello world")
	d1 := &PieceInfo{
		DealID: 5,
		Size:   uint64(len(d1Data)),
	}

	if err := sb.WritePiece(ctx, sector, d1, bytes.NewReader(d1Data)); err != nil {
		t.Fatal(err)
	}

	ag := address.NewForTestGetter()
	_, err = sb.Seal(ctx, sector, ag())
	if err != nil {
		t.Fatal(err)
	}
}

func nodeWithSectorBuilder(t *testing.T) (*tempSectorDirs, *Node, *SectorBuilder, address.Address, uint64) {
	t.Helper()
	require := require.New(t)
	ctx := context.Background()

	nd := MakeOfflineNode(t)

	owner, err := nd.NewAddress()
	require.NoError(err)

	defaultAddr, err := nd.DefaultSenderAddress()
	require.NoError(err)

	tif := th.MakeGenesisFunc(
		th.ActorAccount(owner, types.NewAttoFILFromFIL(1000000)),
		th.ActorAccount(defaultAddr, types.NewAttoFILFromFIL(1000000)),
	)
	require.NoError(nd.ChainMgr.Genesis(ctx, tif))
	require.NoError(nd.Start(ctx))

	pledge := *types.NewBytesAmount(100000)
	coll := *types.NewAttoFILFromFIL(100)

	result := <-RunCreateMiner(t, nd, owner, pledge, core.RequireRandomPeerID(), coll)
	require.NoError(result.Err)
	require.NotNil(result.MinerAddress)

	dirs := newTempSectorDirs()

	sstore := proofs.NewProofTestSectorStore(dirs.SealedDir(), dirs.SealedDir())

	res, err := sstore.GetMaxUnsealedBytesPerSector()
	require.NoError(err)

	sb, err := InitSectorBuilder(nd, *result.MinerAddress, sstore)
	require.NoError(err)

	return dirs, nd, sb, *result.MinerAddress, res.NumBytes
}

func requirePieceInfo(require *require.Assertions, nd *Node, bytes []byte) *PieceInfo {
	data := dag.NewRawNode(bytes)
	err := nd.Blockservice.AddBlock(data)
	require.NoError(err)
	return &PieceInfo{
		Ref:    data.Cid(),
		Size:   uint64(len(bytes)),
		DealID: 0, // FIXME parameterize
	}
}

func TestSectorBuilder(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()

	dirs, nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
	defer dirs.remove()

	var sealingWg sync.WaitGroup
	var sealingErr error
	sealingWg.Add(1)

	sector := sb.curUnsealedSector

	sb.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
		if err != nil {
			panic(err)
		} else if ss != nil && ss.unsealedSectorAccess == sector.unsealedSectorAccess {
			sealingWg.Done()
		}
	}

	requireAddPiece := func(s string) {
		err := sb.AddPiece(ctx, requirePieceInfo(require, nd, []byte(s)))
		assert.NoError(err)
	}

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
	text := "Lorem ipsum dolor sit amet, consectetur turpis duis." // len(text) = 52
	requireAddPiece(text)
	assert.Equal(sector, sb.curUnsealedSector)
	all := text

	metadataMustMatch(require, sb, sector, 1)

	d := requireReadAll(require, sector)
	assert.Equal(all, string(d))
	assert.Nil(sector.sealed)

	text2 := "In gravida quis nisl sit amet interdum. Phasellus metus." // len(text2) = 56
	requireAddPiece(text2)
	assert.Equal(sector, sb.curUnsealedSector)
	all += text2

	d2 := requireReadAll(require, sector)
	assert.Equal(all, string(d2))
	assert.Nil(sector.sealed)

	// persisted and calculated metadata match.
	metadataMustMatch(require, sb, sector, 2)

	text3 := "Ut sit amet mi eget enim scelerisque egestas. Ut volutpat." // len(text3) = 58
	requireAddPiece(text3)

	// wait for sector sealing to complete
	sealingWg.Wait()
	require.NoError(sealingErr)

	assert.NotEqual(sector, sb.curUnsealedSector)

	// persisted and calculated metadata match after a sector is sealed.
	metadataMustMatch(require, sb, sector, 2)

	newSector := sb.curUnsealedSector
	d4 := requireReadAll(require, newSector)
	metadataMustMatch(require, sb, newSector, 1)

	assert.Equal(text3, d4)
	sealed := sector.sealed
	assert.NotNil(sealed)
	assert.Nil(newSector.sealed)

	assert.Equal(sealed.unsealedSectorAccess, sector.unsealedSectorAccess)
	assert.Equal(sealed.pieces, sector.pieces)
	assert.Equal(sealed.numBytes, sector.numBytesUsed)
	_, err = ioutil.ReadFile(sealed.sealedSectorAccess)

	assert.NoError(err)

	meta := sb.curUnsealedSector.SectorMetadata()
	assert.Len(meta.Pieces, 1)
	assert.Equal(uint64(testSectorSize), meta.NumBytesUsed)
	assert.Equal(int(testSectorSize)-len(text3), int(meta.NumBytesFree))

	text4 := "Aliquam molestie porttitor massa at sodales. Vestibulum euismod elit et justo ultrices, ut feugiat justo sodales. Duis ut nullam."
	require.True(len(text4) > int(testSectorSize))

	err = sb.AddPiece(ctx, requirePieceInfo(require, nd, []byte(text4)))
	assert.EqualError(err, ErrPieceTooLarge.Error())
}

func TestSectorBuilderMetadata(t *testing.T) {
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

	t.Run("sealing sector moves metadata", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		dirs, nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer dirs.remove()

		var sealingWg sync.WaitGroup
		var sealingErr error
		sealingWg.Add(1)

		bytesA := make([]byte, 10+(testSectorSize/2))
		bytesB := make([]byte, (testSectorSize/2)-10)

		sector := sb.curUnsealedSector

		sb.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
			if err != nil || ss.unsealedSectorAccess == sector.unsealedSectorAccess {
				sealingErr = err
				sealingWg.Done()
			}
		}

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))
		sectormeta, err := sb.metadataStore.getSectorMetadata(sector.unsealedSectorAccess)
		require.NoError(err)
		require.NotNil(sectormeta)

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))

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
}

func TestSectorStore(t *testing.T) {
	t.Run("it loads a persisted sector", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		dirs, nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer dirs.remove()

		sector := sb.curUnsealedSector

		bytesA := make([]byte, 10+(testSectorSize/2))

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))

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

		dirs, nd, sb, _, testSectorSize := nodeWithSectorBuilder(t)
		defer dirs.remove()

		bytesA := make([]byte, 10+(testSectorSize/2))
		bytesB := make([]byte, (testSectorSize/2)-10)

		sector := sb.curUnsealedSector

		sb.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
			if err != nil || ss.unsealedSectorAccess == sector.unsealedSectorAccess {
				sealingErr = err
				sealingWg.Done()
			}
		}

		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))
		sb.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))

		// wait for sector sealing to complete
		sealingWg.Wait()
		require.NoError(sealingErr)

		require.Equal(1, len(sb.sealedSectors))
		sealedSector := sb.sealedSectors[0]

		loaded, err := sb.metadataStore.getSealedSector(sealedSector.commR)
		require.NoError(err)
		sealedSectorsMustEqual(t, sealedSector, loaded)
	})
}

func TestInitializesSectorBuilderFromPersistedState(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx := context.Background()

	var sealingWg sync.WaitGroup
	var sealingErr error
	sealingWg.Add(1)

	dirs, nd, sbA, minerAddr, testSectorSize := nodeWithSectorBuilder(t)
	defer dirs.remove()

	bytesA := make([]byte, 10+(testSectorSize/2))
	bytesB := make([]byte, (testSectorSize/2)-10)

	sector := sbA.curUnsealedSector

	sbA.OnCommitmentAddedToMempool = func(ss *SealedSector, msgCid *cid.Cid, err error) {
		if err != nil || ss.unsealedSectorAccess == sector.unsealedSectorAccess {
			sealingErr = err
			sealingWg.Done()
		}
	}

	sbA.AddPiece(ctx, requirePieceInfo(require, nd, bytesA))

	// sector builder B should have the same state as sector builder A
	sstore := proofs.NewProofTestSectorStore(dirs.SealedDir(), dirs.SealedDir())

	sbB, err := InitSectorBuilder(nd, minerAddr, sstore)
	require.NoError(err)

	// can't compare sectors with Equal(s1, s2) because their "file" fields will differ
	sectorBuildersMustEqual(t, sbA, sbB)

	// trigger sealing by adding a second piece
	sbA.AddPiece(ctx, requirePieceInfo(require, nd, bytesB))

	// wait for sealing to complete
	sealingWg.Wait()
	require.NoError(sealingErr)

	// sector builder C should have the same state as sector builder A
	sbC, err := InitSectorBuilder(nd, minerAddr, sstore)
	require.NoError(err)

	sectorBuildersMustEqual(t, sbA, sbC)
}

func TestTruncatesUnsealedSectorOnDiskIfMismatch(t *testing.T) {
	t.Run("it truncates the file if file size > metadata size", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, false, true)[0]

		nd.NewAddress() // TODO: default init make an address
		addr, err := nd.DefaultSenderAddress()
		require.NoError(err)

		dirs := newTempSectorDirs()
		defer dirs.remove()

		sstore := proofs.NewProofTestSectorStore(dirs.SealedDir(), dirs.SealedDir())

		sbA, err := InitSectorBuilder(nd, addr, sstore)
		require.NoError(err)

		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 10)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 20)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 50)))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		infoA, err := os.Stat(metaA.UnsealedSectorAccess)
		require.NoError(err)

		// size of file on disk should match what we've persisted as metadata
		require.Equal(int(metaA.NumBytesUsed-metaA.NumBytesFree), int(infoA.Size()))

		// perform an out-of-band write to the file (replaces its contents)
		ioutil.WriteFile(metaA.UnsealedSectorAccess, make([]byte, 90), 0600)

		// initialize a new sector builder (simulates the node restarting)
		sbB, err := InitSectorBuilder(nd, addr, sstore)
		require.NoError(err)

		metaB, err := sbB.metadataStore.getSectorMetadata(sbB.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		infoB, err := os.Stat(metaB.UnsealedSectorAccess)
		require.NoError(err)

		// ensure that the file was truncated to match metadata
		require.Equal(int(metaB.NumBytesUsed-metaB.NumBytesFree), int(infoB.Size()))
		require.Equal(int(infoA.Size()), int(infoB.Size()))
	})

	t.Run("it truncates the metadata if file size < metadata size", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)

		ctx := context.Background()

		nd := MakeNodesStarted(t, 1, false, true)[0]

		nd.NewAddress() // TODO: default init make an address
		addr, err := nd.DefaultSenderAddress()
		require.NoError(err)

		dirs := newTempSectorDirs()
		defer dirs.remove()

		sstore := proofs.NewProofTestSectorStore(dirs.SealedDir(), dirs.SealedDir())

		// Wait a sec, theres no miner here... how can we init a sector builder?
		sbA, err := InitSectorBuilder(nd, addr, sstore)
		require.NoError(err)

		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 10)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 20)))
		sbA.AddPiece(ctx, requirePieceInfo(require, nd, make([]byte, 50)))

		metaA, err := sbA.metadataStore.getSectorMetadata(sbA.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		// truncate the file such that its size < sum(size-of-pieces)
		require.NoError(os.Truncate(metaA.UnsealedSectorAccess, int64(40)))

		// initialize final sector builder
		sbB, err := InitSectorBuilder(nd, addr, sstore)
		require.NoError(err)

		metaB, err := sbA.metadataStore.getSectorMetadata(sbB.curUnsealedSector.unsealedSectorAccess)
		require.NoError(err)

		infoB, err := os.Stat(metaB.UnsealedSectorAccess)
		require.NoError(err)

		// ensure metadata was truncated
		require.Equal(2, len(metaB.Pieces))
		require.Equal(30, int(metaB.NumBytesUsed-metaB.NumBytesFree))

		// ensure that the file was truncated to match metadata
		require.Equal(int(metaB.NumBytesUsed-metaB.NumBytesFree), int(infoB.Size()))
	})
}

func TestProverIdCreation(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	hash := address.Hash([]byte("satoshi"))
	addr := address.NewMainnet(hash)

	id, err := proverID(addr)
	require.NoError(err)

	require.Equal(31, len(id))
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

	builderMeta := sb.SectorBuilderMetadata()
	builderMetaPersisted, err := sb.metadataStore.getSectorBuilderMetadata(sb.MinerAddr)
	require.NoError(err)
	require.Equal(builderMeta, builderMetaPersisted)
}

func pieceInfoMustEqual(t *testing.T, p1 *PieceInfo, p2 *PieceInfo) {
	if p1.Size != p2.Size {
		t.Fatalf("p1.size(%d) != p2.size(%d)\n", p1.Size, p2.Size)
	}

	if p1.DealID != p2.DealID {
		t.Fatalf("p1.DealID(%d) != p2.DealID(%d)\n", p1.DealID, p2.DealID)
	}

	if !p1.Ref.Equals(p2.Ref) {
		t.Fatalf("p1.Ref(%s) != p2.Ref(%s)\n", p1.Ref.String(), p2.Ref.String())
	}
}

func sectorBuildersMustEqual(t *testing.T, sb1 *SectorBuilder, sb2 *SectorBuilder) {
	require := require.New(t)

	require.Equal(sb1.MinerAddr, sb2.MinerAddr)
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
	require.Equal(s1.numBytesFree, s2.numBytesFree)
	require.Equal(s1.numBytesUsed, s2.numBytesUsed)

	sealedSectorsMustEqual(t, s1.sealed, s2.sealed)

	require.Equal(len(s1.pieces), len(s2.pieces))
	for i := 0; i < len(s1.pieces); i++ {
		pieceInfoMustEqual(t, s1.pieces[i], s2.pieces[i])
	}
}

func requireReadAll(require *require.Assertions, sector *UnsealedSector) string {
	data, err := ioutil.ReadFile(sector.unsealedSectorAccess)
	require.NoError(err)

	return string(data)
}
